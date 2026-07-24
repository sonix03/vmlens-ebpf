package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vmlens/vmlens/backend/internal/model"
	"github.com/vmlens/vmlens/backend/internal/realtime"
)

const (
	probeSource              = "vmlens_probe"
	probeTypeConnectivity    = "connectivity_check"
	probeDefaultProtocol     = "tcp"
	probeDefaultPort         = 18081
	probeTargetHistoryWindow = 30 * time.Minute
)

type ConnectionService struct {
	pool *pgxpool.Pool
	hub  *realtime.Hub
}

func NewConnectionService(pool *pgxpool.Pool, hub *realtime.Hub) *ConnectionService {
	return &ConnectionService{pool: pool, hub: hub}
}

func (s *ConnectionService) Targets(ctx context.Context, agentID string) ([]model.ConnectionProbeTarget, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	rows, err := s.pool.Query(ctx, `
		WITH self AS (
			SELECT a.vm_id, COALESCE(host(v.private_ip), '') AS source_ip
			FROM agents a
			JOIN vms v ON v.id = a.vm_id
			WHERE a.id = $1
		),
		peer_flows AS (
			SELECT
				self.vm_id AS source_vm_id,
				self.source_ip,
				CASE
					WHEN f.src_vm_id = self.vm_id THEN f.dst_vm_id
					WHEN f.dst_vm_id = self.vm_id THEN f.src_vm_id
					ELSE NULL
				END AS dest_vm_id,
				MAX(f.last_seen) AS last_seen
			FROM self
			JOIN network_flows f ON f.scope IN ('internal_same_tenant', 'internal_cross_tenant')
				AND (f.src_vm_id = self.vm_id OR f.dst_vm_id = self.vm_id)
				AND f.last_seen >= NOW() - $2::interval
			GROUP BY self.vm_id, self.source_ip, dest_vm_id
		)
		SELECT pf.source_vm_id, pf.source_ip, peer.id, peer.name, COALESCE(host(peer.private_ip), ''),
		       $3::text AS protocol, $4::int AS dest_port, pf.last_seen
		FROM peer_flows pf
		JOIN vms peer ON peer.id = pf.dest_vm_id
		WHERE pf.dest_vm_id IS NOT NULL
		  AND pf.dest_vm_id <> pf.source_vm_id
		  AND peer.private_ip IS NOT NULL
		  AND pf.source_ip <> ''
		ORDER BY pf.last_seen DESC
		LIMIT 200`,
		agentID, fmt.Sprintf("%f seconds", probeTargetHistoryWindow.Seconds()), probeDefaultProtocol, probeDefaultPort)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := []model.ConnectionProbeTarget{}
	for rows.Next() {
		var target model.ConnectionProbeTarget
		if err := rows.Scan(
			&target.SourceVMID, &target.SourceIP, &target.DestVMID, &target.DestName,
			&target.DestIP, &target.Protocol, &target.DestPort, &target.LastSeen,
		); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (s *ConnectionService) RecordProbe(ctx context.Context, event model.ConnectionProbeEvent) error {
	event.AgentID = strings.TrimSpace(event.AgentID)
	event.SourceIP = strings.TrimSpace(event.SourceIP)
	event.DestIP = strings.TrimSpace(event.DestIP)
	event.Protocol = strings.ToLower(strings.TrimSpace(event.Protocol))
	event.Source = strings.TrimSpace(event.Source)
	event.Type = strings.TrimSpace(event.Type)
	event.Error = strings.TrimSpace(event.Error)
	if event.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if event.DestIP == "" {
		return fmt.Errorf("dest_ip is required")
	}
	if event.Protocol == "" {
		event.Protocol = probeDefaultProtocol
	}
	if event.Protocol != "tcp" && event.Protocol != "udp" && event.Protocol != "icmp" {
		return fmt.Errorf("unsupported protocol %q", event.Protocol)
	}
	if event.DestPort <= 0 {
		event.DestPort = probeDefaultPort
	}
	if event.Source == "" {
		event.Source = probeSource
	}
	if event.Type == "" {
		event.Type = probeTypeConnectivity
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if event.Source != probeSource || event.Type != probeTypeConnectivity || event.CountedAsRequest || event.CountedAsUserTraffic {
		return fmt.Errorf("connectivity probe must be uncounted vmlens_probe connectivity_check")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var source resolvedVM
	err = tx.QueryRow(ctx, `
		SELECT v.id, v.name, COALESCE(v.tenant_id, ''), COALESCE(v.agent_id, ''), COALESCE(host(v.private_ip), '')
		FROM agents a
		JOIN vms v ON v.id = a.vm_id
		WHERE a.id = $1`, event.AgentID).Scan(&source.ID, &source.Name, &source.TenantID, &source.AgentID, &event.SourceIP)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("agent %q is not registered", event.AgentID)
	}
	if err != nil {
		return err
	}
	destination, destinationRegistered, err := resolveVMByIP(ctx, tx, event.DestIP)
	if err != nil {
		return err
	}
	var destinationID any
	if destinationRegistered {
		destinationID = destination.ID
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO connection_probes (
			agent_id, src_vm_id, dst_vm_id, src_ip, dst_ip, protocol, dst_port,
			source, probe_type, success, rtt_ms, error, first_seen, last_seen, observed_at
		) VALUES (
			$1, $2, $3, $4::inet, $5::inet, $6, $7, $8, $9, $10, $11, $12, $13, $13, NOW()
		)
		ON CONFLICT (agent_id, src_vm_id, dst_vm_id, dst_ip, protocol, dst_port)
		DO UPDATE SET
			success = EXCLUDED.success,
			rtt_ms = EXCLUDED.rtt_ms,
			error = EXCLUDED.error,
			last_seen = EXCLUDED.last_seen,
			observed_at = NOW()`,
		event.AgentID, source.ID, destinationID, event.SourceIP, event.DestIP,
		event.Protocol, event.DestPort, event.Source, event.Type, event.Success,
		event.RTTMs, nullIfEmpty(event.Error), event.Timestamp)
	if err != nil {
		return fmt.Errorf("record connection probe: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE agents SET last_seen = NOW(), status = 'online' WHERE id = $1`, event.AgentID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE vms SET last_seen = NOW(), status = 'online' WHERE id = $1`, source.ID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if event.Success {
		s.hub.BroadcastLatest("connection.probe", event, 500*time.Millisecond)
	}
	return nil
}
