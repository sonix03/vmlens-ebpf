package service

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vmlens/vmlens/backend/internal/model"
)

type StatsService struct{ pool *pgxpool.Pool }

func NewStatsService(pool *pgxpool.Pool) *StatsService { return &StatsService{pool: pool} }

func (s *StatsService) Summary(ctx context.Context) (model.Summary, error) {
	var summary model.Summary
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM vms),
			(SELECT COUNT(*) FROM vms WHERE status = 'online'),
			(SELECT COUNT(*) FROM vms WHERE status = 'stale'),
			(SELECT COUNT(*) FROM vms WHERE status = 'offline'),
			(SELECT COUNT(*) FROM network_flows),
			(SELECT COUNT(*) FROM network_flows WHERE scope IN ('internal_same_tenant', 'internal_cross_tenant', 'unknown_internal')),
			(SELECT COUNT(*) FROM network_flows WHERE scope = 'external_public'),
			COALESCE((SELECT SUM(bytes_sent + bytes_received) FROM network_flows WHERE scope IN ('internal_same_tenant', 'internal_cross_tenant', 'unknown_internal')), 0),
			COALESCE((SELECT SUM(bytes_sent) FROM network_flows WHERE scope IN ('internal_same_tenant', 'internal_cross_tenant', 'unknown_internal')), 0),
			COALESCE((SELECT SUM(bytes_received) FROM network_flows WHERE scope IN ('internal_same_tenant', 'internal_cross_tenant', 'unknown_internal')), 0),
			COALESCE((SELECT SUM(bytes_sent + bytes_received) FROM network_flows WHERE scope = 'external_public'), 0),
			COALESCE((SELECT SUM(bytes_sent) FROM network_flows WHERE scope = 'external_public'), 0),
			COALESCE((SELECT SUM(bytes_received) FROM network_flows WHERE scope = 'external_public'), 0),
			(SELECT COUNT(*) FROM unknown_internal_hosts WHERE resolved_vm_id IS NULL),
			COALESCE((SELECT SUM(request_count) FROM network_flows), 0),
			COALESCE((SELECT SUM(request_count) FROM flow_observations WHERE observed_at >= NOW() - INTERVAL '60 seconds'), 0),
			COALESCE((SELECT SUM(request_count)::double precision / 60 FROM flow_observations WHERE observed_at >= NOW() - INTERVAL '60 seconds'), 0),
			COALESCE((SELECT SUM(connection_count)::double precision / 60 FROM flow_observations WHERE observed_at >= NOW() - INTERVAL '60 seconds'), 0)
	`).Scan(&summary.TotalVMs, &summary.OnlineVMs, &summary.StaleVMs, &summary.OfflineVMs,
		&summary.TotalFlows, &summary.InternalFlows, &summary.ExternalFlows,
		&summary.InternalBytes, &summary.InternalSent, &summary.InternalRecv,
		&summary.ExternalBytes, &summary.ExternalSent, &summary.ExternalRecv, &summary.UnknownInternal,
		&summary.RequestTotal, &summary.RequestLastMinute, &summary.RequestsPerSec, &summary.ConnectionsPerSec)
	summary.UpdatedAt = time.Now().UTC()
	return summary, err
}

func (s *StatsService) TopTalkers(ctx context.Context, limit int) ([]model.TopTalker, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx, `
		SELECT v.id, v.name,
		       COALESCE(SUM(f.bytes_sent), 0)::bigint,
		       COALESCE(SUM(f.bytes_received), 0)::bigint,
		       COALESCE(SUM(f.bytes_sent + f.bytes_received), 0)::bigint AS total
		FROM vms v
		LEFT JOIN network_flows f ON f.src_vm_id = v.id
		GROUP BY v.id, v.name
		ORDER BY total DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.TopTalker{}
	for rows.Next() {
		var talker model.TopTalker
		if err := rows.Scan(&talker.VMID, &talker.Name, &talker.BytesSent, &talker.BytesReceived, &talker.TotalBytes); err != nil {
			return nil, err
		}
		result = append(result, talker)
	}
	return result, rows.Err()
}
