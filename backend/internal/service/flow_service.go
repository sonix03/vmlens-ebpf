package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vmlens/vmlens/backend/internal/model"
	"github.com/vmlens/vmlens/backend/internal/realtime"
	flowmetrics "github.com/vmlens/vmlens/backend/internal/telemetry/metrics"
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
	if err := flowmetrics.ValidateFlowEvent(&event); err != nil {
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
				request_count, error_count, retransmission_count, avg_rtt_ms, avg_app_delay_ms,
				first_seen, last_seen, last_error_at, interface_name,
				http_1xx_count, http_2xx_count, http_3xx_count, http_4xx_count, http_5xx_count, last_http_status
			) VALUES ($1, $2, $3, $4::inet, $5::inet, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)
			RETURNING id::text`, event.AgentID, source.ID, destinationID, event.SrcIP, event.DstIP,
			event.SrcPort, event.DstPort, event.Protocol, event.Direction, scope, event.BytesSent, event.BytesReceived,
			event.Packets, event.ConnectionCount, event.RequestCount, event.ErrorCount, event.Retransmissions, event.AvgRTTMs, event.AvgAppDelayMs,
			event.FirstSeen, event.LastSeen,
			flowmetrics.LastErrorAtArg(event.ErrorCount, observedAt), nullIfEmpty(event.Interface),
			event.HTTP1xx, event.HTTP2xx, event.HTTP3xx, event.HTTP4xx, event.HTTP5xx,
			nullIfZeroStatus(event.LastHTTPStatus)).Scan(&flowID)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE network_flows SET
				bytes_sent = bytes_sent + $2,
				bytes_received = bytes_received + $3,
				packets = packets + $4,
				connection_count = connection_count + $5,
				request_count = request_count + $6,
				error_count = error_count + $7,
				retransmission_count = retransmission_count + $8,
				avg_rtt_ms = CASE
					WHEN $9 > 0 AND avg_rtt_ms > 0 THEN (avg_rtt_ms + $9) / 2
					WHEN $9 > 0 THEN $9
					ELSE avg_rtt_ms
				END,
				avg_app_delay_ms = CASE
					WHEN $10 > 0 AND avg_app_delay_ms > 0 THEN (avg_app_delay_ms + $10) / 2
					WHEN $10 > 0 THEN $10
					ELSE avg_app_delay_ms
				END,
				first_seen = LEAST(first_seen, $11),
				last_seen = GREATEST(last_seen, $12),
				agent_id = $13,
				interface_name = COALESCE($14, interface_name),
				last_error_at = CASE
					WHEN $7 > 0 THEN GREATEST(COALESCE(last_error_at, $15), $15)
					ELSE last_error_at
				END,
				observed_at = $15,
				http_1xx_count = http_1xx_count + $16,
				http_2xx_count = http_2xx_count + $17,
				http_3xx_count = http_3xx_count + $18,
				http_4xx_count = http_4xx_count + $19,
				http_5xx_count = http_5xx_count + $20,
				last_http_status = COALESCE($21, last_http_status)
			WHERE id = $1::uuid`, flowID, event.BytesSent, event.BytesReceived, event.Packets,
			event.ConnectionCount, event.RequestCount, event.ErrorCount, event.Retransmissions, event.AvgRTTMs, event.AvgAppDelayMs,
			event.FirstSeen, event.LastSeen, event.AgentID, nullIfEmpty(event.Interface), observedAt,
			event.HTTP1xx, event.HTTP2xx, event.HTTP3xx, event.HTTP4xx, event.HTTP5xx,
			nullIfZeroStatus(event.LastHTTPStatus))
	}
	if err != nil {
		return model.Flow{}, fmt.Errorf("aggregate flow: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO flow_observations (
			flow_id, agent_id, src_vm_id, dst_vm_id, src_ip, dst_ip, src_port, dst_port,
			protocol, direction, scope, bytes_sent, bytes_received, packets, connection_count,
			request_count, error_count, retransmission_count, avg_rtt_ms, avg_app_delay_ms,
			first_seen, last_seen, observed_at,
			http_1xx_count, http_2xx_count, http_3xx_count, http_4xx_count, http_5xx_count, last_http_status
		) VALUES ($1::uuid, $2, $3, $4, $5::inet, $6::inet, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)`,
		flowID, event.AgentID, source.ID, destinationID, event.SrcIP, event.DstIP,
		event.SrcPort, event.DstPort, event.Protocol, event.Direction, scope,
		event.BytesSent, event.BytesReceived, event.Packets, event.ConnectionCount,
		event.RequestCount, event.ErrorCount, event.Retransmissions, event.AvgRTTMs, event.AvgAppDelayMs,
		event.FirstSeen, event.LastSeen, observedAt,
		event.HTTP1xx, event.HTTP2xx, event.HTTP3xx, event.HTTP4xx, event.HTTP5xx,
		nullIfZeroStatus(event.LastHTTPStatus)); err != nil {
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
		Retransmissions: event.Retransmissions, AvgRTTMs: event.AvgRTTMs, AvgAppDelayMs: event.AvgAppDelayMs,
		HTTP1xx: event.HTTP1xx, HTTP2xx: event.HTTP2xx, HTTP3xx: event.HTTP3xx,
		HTTP4xx: event.HTTP4xx, HTTP5xx: event.HTTP5xx, LastHTTPStatus: event.LastHTTPStatus,
		RequestsPerSec:    flowmetrics.RatePerSecond(event.RequestCount, event.FirstSeen, event.LastSeen),
		ConnectionsPerSec: flowmetrics.RatePerSecond(event.ConnectionCount, event.FirstSeen, event.LastSeen),
		FirstSeen:         event.FirstSeen,
		LastSeen:          event.LastSeen, ObservedAt: observedAt, LastErrorAt: flowmetrics.LastErrorAtPtr(event.ErrorCount, observedAt), InterfaceName: event.Interface,
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
		       request_count, error_count, retransmission_count, avg_rtt_ms, avg_app_delay_ms,
		       first_seen, last_seen, observed_at, last_error_at, COALESCE(interface_name, ''), created_at,
		       http_1xx_count, http_2xx_count, http_3xx_count, http_4xx_count, http_5xx_count,
		       COALESCE(last_http_status, 0)
		FROM network_flows ORDER BY observed_at DESC LIMIT $1`, limit)
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
			&flow.Packets, &flow.ConnectionCount, &flow.RequestCount, &flow.ErrorCount, &flow.Retransmissions,
			&flow.AvgRTTMs, &flow.AvgAppDelayMs, &flow.FirstSeen, &flow.LastSeen,
			&flow.ObservedAt, &lastErrorAt, &flow.InterfaceName, &flow.CreatedAt,
			&flow.HTTP1xx, &flow.HTTP2xx, &flow.HTTP3xx, &flow.HTTP4xx, &flow.HTTP5xx,
			&flow.LastHTTPStatus); err != nil {
			return nil, err
		}
		if lastErrorAt.Valid {
			flow.LastErrorAt = &lastErrorAt.Time
		}
		flow.Service, flow.ServicePort = classifyService(flow.Protocol, flow.Direction, flow.SrcPort, flow.DstPort)
		flow.RequestsPerSec = flowmetrics.RatePerSecond(flow.RequestCount, flow.FirstSeen, flow.LastSeen)
		flow.ConnectionsPerSec = flowmetrics.RatePerSecond(flow.ConnectionCount, flow.FirstSeen, flow.LastSeen)
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
	queryLimit := limit * 20
	if queryLimit < 200 {
		queryLimit = 200
	}
	if queryLimit > 5000 {
		queryLimit = 5000
	}
	rows, err := s.pool.Query(ctx, `
		SELECT f.id::text, f.src_vm_id, COALESCE(observer.name, ''), host(f.src_ip),
		       f.dst_vm_id, COALESCE(peer.name, ''), host(f.dst_ip),
		       COALESCE(f.src_port, 0), COALESCE(f.dst_port, 0), f.protocol, f.direction, f.scope,
		       f.bytes_sent, f.bytes_received, f.connection_count, f.request_count, f.error_count,
		       f.retransmission_count, f.avg_rtt_ms, f.avg_app_delay_ms,
		       f.first_seen, f.last_seen, f.observed_at
		FROM flow_observations f
		JOIN vms observer ON observer.id = f.src_vm_id
		JOIN vms peer ON peer.id = f.dst_vm_id
		WHERE f.scope IN ('internal_same_tenant', 'internal_cross_tenant')
		  AND (f.request_count > 0 OR f.connection_count > 0 OR f.error_count > 0)
		  AND f.observed_at >= NOW() - $2::interval
		  AND (cardinality($3::int[]) = 0 OR COALESCE(f.src_port, 0) = ANY($3::int[]) OR COALESCE(f.dst_port, 0) = ANY($3::int[]))
		  AND NOT (COALESCE(f.src_port, 0) = ANY($4::int[]) OR COALESCE(f.dst_port, 0) = ANY($4::int[]))
		  AND NOT (host(f.src_ip) = ANY($5::text[]) OR host(f.dst_ip) = ANY($5::text[]))
		ORDER BY f.observed_at DESC
		LIMIT $1`,
		queryLimit,
		fmt.Sprintf("%f seconds", window.Seconds()),
		s.visibility.AllowedPorts,
		s.visibility.ExcludedPorts,
		s.visibility.ExcludedIPs,
	)
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
			&activity.ErrorCount, &activity.Retransmissions, &activity.AvgRTTMs, &activity.AvgAppDelayMs,
			&activity.FirstSeen, &activity.LastSeen, &activity.ObservedAt,
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
		activity.RequestsPerSec = flowmetrics.RatePerSecond(activity.RequestCount, activity.FirstSeen, activity.LastSeen)
		activity.ConnectionsPerSec = flowmetrics.RatePerSecond(activity.ConnectionCount, activity.FirstSeen, activity.LastSeen)
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
		if len(result) >= limit {
			break
		}
	}
	return result, rows.Err()
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
