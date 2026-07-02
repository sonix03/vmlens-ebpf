package service

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vmlens/vmlens/backend/internal/model"
	"github.com/vmlens/vmlens/backend/internal/realtime"
)

type FlowService struct {
	pool       *pgxpool.Pool
	classifier *Classifier
	hub        *realtime.Hub
}

func NewFlowService(pool *pgxpool.Pool, classifier *Classifier, hub *realtime.Hub) *FlowService {
	return &FlowService{pool: pool, classifier: classifier, hub: hub}
}

func (s *FlowService) Ingest(ctx context.Context, event model.FlowEvent) (model.Flow, error) {
	if err := validateFlow(&event); err != nil {
		return model.Flow{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.Flow{}, err
	}
	defer tx.Rollback(ctx)

	var source resolvedVM
	err = tx.QueryRow(ctx, `
		SELECT v.id, v.name, COALESCE(v.tenant_id, ''), COALESCE(v.agent_id, '')
		FROM agents a JOIN vms v ON v.id = a.vm_id
		WHERE a.id = $1`, event.AgentID).Scan(&source.ID, &source.Name, &source.TenantID, &source.AgentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Flow{}, fmt.Errorf("agent %q is not registered", event.AgentID)
	}
	if err != nil {
		return model.Flow{}, err
	}

	destination, destinationRegistered, err := resolveVMByIP(ctx, tx, event.DstIP)
	if err != nil {
		return model.Flow{}, err
	}
	scope := s.classifier.Scope(source.TenantID, destination.TenantID, destinationRegistered, event.DstIP)
	var destinationID any
	if destinationRegistered {
		destinationID = destination.ID
	}

	switch scope {
	case "unknown_internal":
		_, err = tx.Exec(ctx, `
			INSERT INTO unknown_internal_hosts (ip, first_seen, last_seen)
			VALUES ($1::inet, $2, $3)
			ON CONFLICT (ip) DO UPDATE SET last_seen = GREATEST(unknown_internal_hosts.last_seen, EXCLUDED.last_seen)`,
			event.DstIP, event.FirstSeen, event.LastSeen)
	case "external_public":
		_, err = tx.Exec(ctx, `
			INSERT INTO external_hosts (ip, first_seen, last_seen)
			VALUES ($1::inet, $2, $3)
			ON CONFLICT (ip) DO UPDATE SET last_seen = GREATEST(external_hosts.last_seen, EXCLUDED.last_seen)`,
			event.DstIP, event.FirstSeen, event.LastSeen)
	}
	if err != nil {
		return model.Flow{}, err
	}

	// Serialize a single aggregate key. This avoids duplicate graph edges under
	// concurrent ingestion without depending on NULL equality in a unique index.
	lockKey := fmt.Sprintf("%s|%s|%s|%s|%d|%s", source.ID, valueOr(event.SrcIP, source.ID), valueOr(destination.ID, event.DstIP), event.Protocol, event.DstPort, scope)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return model.Flow{}, err
	}

	var flowID string
	err = tx.QueryRow(ctx, `
		SELECT id::text FROM network_flows
		WHERE src_vm_id IS NOT DISTINCT FROM $1
		  AND dst_vm_id IS NOT DISTINCT FROM $2
		  AND src_ip = $3::inet AND dst_ip = $4::inet
		  AND protocol = $5 AND dst_port = $6 AND scope = $7
		LIMIT 1 FOR UPDATE`, source.ID, destinationID, event.SrcIP, event.DstIP, event.Protocol, event.DstPort, scope).Scan(&flowID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return model.Flow{}, err
	}

	if flowID == "" {
		err = tx.QueryRow(ctx, `
			INSERT INTO network_flows (
				agent_id, src_vm_id, dst_vm_id, src_ip, dst_ip, src_port, dst_port,
				protocol, direction, scope, bytes_sent, bytes_received, packets, connection_count,
				first_seen, last_seen, interface_name
			) VALUES ($1, $2, $3, $4::inet, $5::inet, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
			RETURNING id::text`, event.AgentID, source.ID, destinationID, event.SrcIP, event.DstIP,
			event.SrcPort, event.DstPort, event.Protocol, event.Direction, scope, event.BytesSent, event.BytesReceived,
			event.Packets, event.ConnectionCount, event.FirstSeen, event.LastSeen, nullIfEmpty(event.Interface)).Scan(&flowID)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE network_flows SET
				bytes_sent = bytes_sent + $2,
				bytes_received = bytes_received + $3,
				packets = packets + $4,
				connection_count = connection_count + $5,
				first_seen = LEAST(first_seen, $6),
				last_seen = GREATEST(last_seen, $7),
				agent_id = $8,
				interface_name = COALESCE($9, interface_name)
			WHERE id = $1::uuid`, flowID, event.BytesSent, event.BytesReceived, event.Packets,
			event.ConnectionCount, event.FirstSeen, event.LastSeen, event.AgentID, nullIfEmpty(event.Interface))
	}
	if err != nil {
		return model.Flow{}, fmt.Errorf("aggregate flow: %w", err)
	}

	if _, err := tx.Exec(ctx, `UPDATE agents SET last_seen = NOW(), status = 'online' WHERE id = $1`, event.AgentID); err != nil {
		return model.Flow{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE vms SET last_seen = NOW(), status = 'online' WHERE id = $1`, source.ID); err != nil {
		return model.Flow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Flow{}, err
	}

	flow := model.Flow{
		ID: flowID, AgentID: event.AgentID, SrcVMID: source.ID, SrcIP: event.SrcIP,
		DstIP: event.DstIP, SrcPort: event.SrcPort, DstPort: event.DstPort,
		Protocol: event.Protocol, Direction: event.Direction, Scope: scope, BytesSent: event.BytesSent,
		BytesReceived: event.BytesReceived, Packets: event.Packets,
		ConnectionCount: event.ConnectionCount, FirstSeen: event.FirstSeen,
		LastSeen: event.LastSeen, InterfaceName: event.Interface,
	}
	if destinationRegistered {
		flow.DstVMID = destination.ID
	}
	s.hub.Broadcast("flow.updated", flow)
	return flow, nil
}

func (s *FlowService) List(ctx context.Context, limit int) ([]model.Flow, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, COALESCE(agent_id, ''), COALESCE(src_vm_id, ''), COALESCE(dst_vm_id, ''),
		       host(src_ip), host(dst_ip), COALESCE(src_port, 0), COALESCE(dst_port, 0),
		       protocol, direction, scope, bytes_sent, bytes_received, packets, connection_count,
		       first_seen, last_seen, COALESCE(interface_name, ''), created_at
		FROM network_flows ORDER BY last_seen DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	flows := []model.Flow{}
	for rows.Next() {
		var flow model.Flow
		if err := rows.Scan(&flow.ID, &flow.AgentID, &flow.SrcVMID, &flow.DstVMID, &flow.SrcIP, &flow.DstIP,
			&flow.SrcPort, &flow.DstPort, &flow.Protocol, &flow.Direction, &flow.Scope, &flow.BytesSent, &flow.BytesReceived,
			&flow.Packets, &flow.ConnectionCount, &flow.FirstSeen, &flow.LastSeen, &flow.InterfaceName, &flow.CreatedAt); err != nil {
			return nil, err
		}
		flows = append(flows, flow)
	}
	return flows, rows.Err()
}

func validateFlow(event *model.FlowEvent) error {
	event.AgentID = strings.TrimSpace(event.AgentID)
	event.Protocol = strings.ToLower(strings.TrimSpace(event.Protocol))
	event.Direction = strings.ToLower(strings.TrimSpace(event.Direction))
	if event.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if event.Protocol != "tcp" && event.Protocol != "udp" {
		return fmt.Errorf("protocol must be tcp or udp")
	}
	if event.Direction == "" {
		event.Direction = "egress"
	}
	if event.Direction != "ingress" && event.Direction != "egress" {
		return fmt.Errorf("direction must be ingress or egress")
	}
	if _, err := netip.ParseAddr(event.SrcIP); err != nil {
		return fmt.Errorf("invalid src_ip: %w", err)
	}
	if _, err := netip.ParseAddr(event.DstIP); err != nil {
		return fmt.Errorf("invalid dst_ip: %w", err)
	}
	if event.SrcPort < 0 || event.SrcPort > 65535 || event.DstPort < 0 || event.DstPort > 65535 {
		return fmt.Errorf("ports must be between 0 and 65535")
	}
	if event.BytesSent < 0 || event.BytesReceived < 0 || event.Packets < 0 || event.ConnectionCount < 0 {
		return fmt.Errorf("flow counters cannot be negative")
	}
	now := time.Now().UTC()
	if event.FirstSeen.IsZero() {
		event.FirstSeen = now
	}
	if event.LastSeen.IsZero() {
		event.LastSeen = event.FirstSeen
	}
	if event.LastSeen.Before(event.FirstSeen) {
		return fmt.Errorf("last_seen cannot be before first_seen")
	}
	return nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
