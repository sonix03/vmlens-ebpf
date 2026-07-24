package service

import (
	"context"
	"database/sql"
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
	visibility GraphVisibility
}

func NewFlowService(pool *pgxpool.Pool, classifier *Classifier, hub *realtime.Hub, visibility GraphVisibility) *FlowService {
	return &FlowService{pool: pool, classifier: classifier, hub: hub, visibility: visibility}
}

func (s *FlowService) Ingest(ctx context.Context, event model.FlowEvent) (model.Flow, error) {
	if err := validateFlow(&event); err != nil {
		return model.Flow{}, err
	}
	observedAt := time.Now().UTC()
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
	case ScopeUnknownInternal:
		_, err = tx.Exec(ctx, `
			INSERT INTO unknown_internal_hosts (ip, first_seen, last_seen)
			VALUES ($1::inet, $2, $3)
			ON CONFLICT (ip) DO UPDATE SET last_seen = GREATEST(unknown_internal_hosts.last_seen, EXCLUDED.last_seen)`,
			event.DstIP, event.FirstSeen, event.LastSeen)
	case ScopeExternalPublic, ScopeExternalPrivate:
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
	lockKey := fmt.Sprintf("%s|%s|%s|%s|%s|%d|%s", source.ID, valueOr(event.SrcIP, source.ID), valueOr(destination.ID, event.DstIP), event.Protocol, event.Direction, event.DstPort, scope)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return model.Flow{}, err
	}

	var flowID string
	err = tx.QueryRow(ctx, `
		SELECT id::text FROM network_flows
		WHERE src_vm_id IS NOT DISTINCT FROM $1
		  AND dst_vm_id IS NOT DISTINCT FROM $2
		  AND src_ip = $3::inet AND dst_ip = $4::inet
		  AND protocol = $5 AND dst_port = $6 AND scope = $7 AND direction = $8
		LIMIT 1 FOR UPDATE`, source.ID, destinationID, event.SrcIP, event.DstIP, event.Protocol, event.DstPort, scope, event.Direction).Scan(&flowID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return model.Flow{}, err
	}

	if flowID == "" {
		err = tx.QueryRow(ctx, `
			INSERT INTO network_flows (
				agent_id, src_vm_id, dst_vm_id, src_ip, dst_ip, src_port, dst_port,
				protocol, direction, scope, bytes_sent, bytes_received, packets, connection_count,
				request_count, error_count, first_seen, last_seen, last_error_at, interface_name
			) VALUES ($1, $2, $3, $4::inet, $5::inet, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
			RETURNING id::text`, event.AgentID, source.ID, destinationID, event.SrcIP, event.DstIP,
			event.SrcPort, event.DstPort, event.Protocol, event.Direction, scope, event.BytesSent, event.BytesReceived,
			event.Packets, event.ConnectionCount, event.RequestCount, event.ErrorCount, event.FirstSeen, event.LastSeen,
			lastErrorAtArg(event.ErrorCount, observedAt), nullIfEmpty(event.Interface)).Scan(&flowID)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE network_flows SET
				bytes_sent = bytes_sent + $2,
				bytes_received = bytes_received + $3,
				packets = packets + $4,
				connection_count = connection_count + $5,
				request_count = request_count + $6,
				error_count = error_count + $7,
				first_seen = LEAST(first_seen, $8),
				last_seen = GREATEST(last_seen, $9),
				agent_id = $10,
				interface_name = COALESCE($11, interface_name),
				last_error_at = CASE
					WHEN $7 > 0 THEN GREATEST(COALESCE(last_error_at, $12), $12)
					ELSE last_error_at
				END,
				observed_at = $12
			WHERE id = $1::uuid`, flowID, event.BytesSent, event.BytesReceived, event.Packets,
			event.ConnectionCount, event.RequestCount, event.ErrorCount, event.FirstSeen, event.LastSeen,
			event.AgentID, nullIfEmpty(event.Interface), observedAt)
	}
	if err != nil {
		return model.Flow{}, fmt.Errorf("aggregate flow: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO flow_observations (
			flow_id, agent_id, src_vm_id, dst_vm_id, src_ip, dst_ip, src_port, dst_port,
			protocol, direction, scope, bytes_sent, bytes_received, packets, connection_count,
			request_count, error_count, first_seen, last_seen, observed_at
		) VALUES ($1::uuid, $2, $3, $4, $5::inet, $6::inet, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`,
		flowID, event.AgentID, source.ID, destinationID, event.SrcIP, event.DstIP,
		event.SrcPort, event.DstPort, event.Protocol, event.Direction, scope,
		event.BytesSent, event.BytesReceived, event.Packets, event.ConnectionCount,
		event.RequestCount, event.ErrorCount, event.FirstSeen, event.LastSeen, observedAt); err != nil {
		return model.Flow{}, fmt.Errorf("record flow observation: %w", err)
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

	serviceName, servicePort := classifyService(event.Protocol, event.Direction, event.SrcPort, event.DstPort)
	flow := model.Flow{
		ID: flowID, AgentID: event.AgentID, SrcVMID: source.ID, SrcIP: event.SrcIP,
		DstIP: event.DstIP, SrcPort: event.SrcPort, DstPort: event.DstPort,
		Protocol: event.Protocol, Direction: event.Direction, Scope: scope, Service: serviceName, ServicePort: servicePort, BytesSent: event.BytesSent,
		BytesReceived: event.BytesReceived, Packets: event.Packets,
		ConnectionCount: event.ConnectionCount, RequestCount: event.RequestCount, ErrorCount: event.ErrorCount,
		RequestsPerSec:    ratePerSecond(event.RequestCount, event.FirstSeen, event.LastSeen),
		ConnectionsPerSec: ratePerSecond(event.ConnectionCount, event.FirstSeen, event.LastSeen),
		FirstSeen:         event.FirstSeen,
		LastSeen:          event.LastSeen, ObservedAt: observedAt, LastErrorAt: lastErrorAtPtr(event.ErrorCount, observedAt), InterfaceName: event.Interface,
	}
	if destinationRegistered {
		flow.DstVMID = destination.ID
	}
	s.hub.BroadcastLatest("flow.updated", flow, 500*time.Millisecond)
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
		       request_count, error_count, first_seen, last_seen, observed_at, last_error_at, COALESCE(interface_name, ''), created_at
		FROM network_flows ORDER BY last_seen DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	flows := []model.Flow{}
	for rows.Next() {
		var flow model.Flow
		var lastErrorAt sql.NullTime
		if err := rows.Scan(&flow.ID, &flow.AgentID, &flow.SrcVMID, &flow.DstVMID, &flow.SrcIP, &flow.DstIP,
			&flow.SrcPort, &flow.DstPort, &flow.Protocol, &flow.Direction, &flow.Scope, &flow.BytesSent, &flow.BytesReceived,
			&flow.Packets, &flow.ConnectionCount, &flow.RequestCount, &flow.ErrorCount, &flow.FirstSeen, &flow.LastSeen,
			&flow.ObservedAt, &lastErrorAt, &flow.InterfaceName, &flow.CreatedAt); err != nil {
			return nil, err
		}
		if lastErrorAt.Valid {
			flow.LastErrorAt = &lastErrorAt.Time
		}
		flow.Service, flow.ServicePort = classifyService(flow.Protocol, flow.Direction, flow.SrcPort, flow.DstPort)
		flow.RequestsPerSec = ratePerSecond(flow.RequestCount, flow.FirstSeen, flow.LastSeen)
		flow.ConnectionsPerSec = ratePerSecond(flow.ConnectionCount, flow.FirstSeen, flow.LastSeen)
		flows = append(flows, flow)
	}
	return flows, rows.Err()
}

func (s *FlowService) ListInternalActivity(ctx context.Context, limit int, window time.Duration) ([]model.InternalActivity, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if window <= 0 {
		window = 5 * time.Minute
	}
	rows, err := s.pool.Query(ctx, `
		SELECT f.id::text, f.src_vm_id, COALESCE(observer.name, ''), host(f.src_ip),
		       f.dst_vm_id, COALESCE(peer.name, ''), host(f.dst_ip),
		       COALESCE(f.src_port, 0), COALESCE(f.dst_port, 0), f.protocol, f.direction, f.scope,
		       f.bytes_sent, f.bytes_received, f.connection_count, f.request_count, f.error_count, f.first_seen, f.last_seen, f.observed_at
		FROM flow_observations f
		JOIN vms observer ON observer.id = f.src_vm_id
		JOIN vms peer ON peer.id = f.dst_vm_id
		WHERE f.scope IN ('internal_same_tenant', 'internal_cross_tenant')
		  AND (f.request_count > 0 OR f.connection_count > 0 OR f.error_count > 0)
		  AND f.observed_at >= NOW() - $2::interval
		ORDER BY f.observed_at DESC
		LIMIT $1`, limit, fmt.Sprintf("%f seconds", window.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.InternalActivity, 0, limit)
	for rows.Next() {
		var activity model.InternalActivity
		if err := rows.Scan(
			&activity.ID, &activity.ObserverVMID, &activity.ObserverName, &activity.ObserverIP,
			&activity.PeerVMID, &activity.PeerName, &activity.PeerIP,
			&activity.LocalPort, &activity.PeerPort, &activity.Protocol, &activity.Direction, &activity.Scope,
			&activity.BytesSent, &activity.BytesReceived, &activity.ConnectionCount, &activity.RequestCount,
			&activity.ErrorCount, &activity.FirstSeen, &activity.LastSeen, &activity.ObservedAt,
		); err != nil {
			return nil, err
		}
		activity.Service, activity.ServicePort = classifyService(activity.Protocol, activity.Direction, activity.LocalPort, activity.PeerPort)
		if activity.ObserverIP == activity.PeerIP || (activity.ObserverVMID != "" && activity.ObserverVMID == activity.PeerVMID) {
			continue
		}
		if hiddenByGraphVisibility(s.visibility, activity.LocalPort, activity.PeerPort, activity.ObserverIP, activity.PeerIP) ||
			hiddenByServicePort(s.visibility, activity.ServicePort) {
			continue
		}
		activity.RequestsPerSec = ratePerSecond(activity.RequestCount, activity.FirstSeen, activity.LastSeen)
		activity.ConnectionsPerSec = ratePerSecond(activity.ConnectionCount, activity.FirstSeen, activity.LastSeen)
		activity.ObserverName = valueOr(activity.ObserverName, activity.ObserverIP)
		activity.PeerName = valueOr(activity.PeerName, activity.PeerIP)
		if shouldFlipServiceResponse(activity.LocalPort, activity.PeerPort) {
			activity.SourceVMID, activity.SourceName, activity.SourceIP = activity.PeerVMID, activity.PeerName, activity.PeerIP
			activity.DestinationVMID, activity.DestinationName, activity.DestinationIP = activity.ObserverVMID, activity.ObserverName, activity.ObserverIP
			activity.BytesSent, activity.BytesReceived = activity.BytesReceived, activity.BytesSent
		} else {
			activity.SourceVMID, activity.SourceName, activity.SourceIP = activity.ObserverVMID, activity.ObserverName, activity.ObserverIP
			activity.DestinationVMID, activity.DestinationName, activity.DestinationIP = activity.PeerVMID, activity.PeerName, activity.PeerIP
		}
		result = append(result, activity)
	}
	return result, rows.Err()
}

func validateFlow(event *model.FlowEvent) error {
	event.AgentID = strings.TrimSpace(event.AgentID)
	event.Protocol = strings.ToLower(strings.TrimSpace(event.Protocol))
	event.Direction = strings.ToLower(strings.TrimSpace(event.Direction))
	if event.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if event.Protocol != "tcp" && event.Protocol != "udp" && event.Protocol != "icmp" {
		return fmt.Errorf("protocol must be tcp, udp, or icmp")
	}
	if event.Protocol == "icmp" {
		event.SrcPort = 0
		event.DstPort = 0
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
	if event.BytesSent < 0 || event.BytesReceived < 0 || event.Packets < 0 || event.ConnectionCount < 0 || event.RequestCount < 0 || event.ErrorCount < 0 {
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
	if event.RequestCount == 0 {
		event.RequestCount = inferRequestCount(*event)
	}
	return nil
}

func lastErrorAtArg(errorCount int64, observedAt time.Time) any {
	if errorCount <= 0 {
		return nil
	}
	return observedAt
}

func lastErrorAtPtr(errorCount int64, observedAt time.Time) *time.Time {
	if errorCount <= 0 {
		return nil
	}
	value := observedAt
	return &value
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func inferRequestCount(event model.FlowEvent) int64 {
	if event.ErrorCount > 0 {
		return 0
	}
	if event.ConnectionCount > 0 {
		return event.ConnectionCount
	}
	if event.Protocol == "udp" || event.Protocol == "icmp" {
		switch event.Direction {
		case "egress":
			if event.BytesSent > 0 {
				return 1
			}
		case "ingress":
			if event.BytesReceived > 0 {
				return 1
			}
		}
	}
	return 0
}

func ratePerSecond(count int64, firstSeen, lastSeen time.Time) float64 {
	if count <= 0 {
		return 0
	}
	seconds := lastSeen.Sub(firstSeen).Seconds()
	if seconds < 1 {
		seconds = 1
	}
	return float64(count) / seconds
}
