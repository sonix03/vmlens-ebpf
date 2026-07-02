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
			COALESCE((SELECT SUM(bytes_sent + bytes_received) FROM network_flows WHERE scope IN ('internal_same_tenant', 'internal_cross_tenant')), 0),
			COALESCE((SELECT SUM(bytes_sent + bytes_received) FROM network_flows WHERE scope = 'external_public'), 0),
			(SELECT COUNT(*) FROM unknown_internal_hosts WHERE resolved_vm_id IS NULL)
	`).Scan(&summary.TotalVMs, &summary.OnlineVMs, &summary.StaleVMs, &summary.OfflineVMs,
		&summary.TotalFlows, &summary.InternalBytes, &summary.ExternalBytes, &summary.UnknownInternal)
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
