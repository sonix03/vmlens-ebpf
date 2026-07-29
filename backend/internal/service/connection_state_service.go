package service

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vmlens/vmlens/backend/internal/model"
)

const defaultConnectionViewWindow = 15 * time.Minute

type ConnectionStateService struct {
	pool         *pgxpool.Pool
	activeWindow time.Duration
	slowRTTMS    float64
}

func NewConnectionStateService(pool *pgxpool.Pool, activeWindow time.Duration) *ConnectionStateService {
	if activeWindow <= 0 {
		activeWindow = 5 * time.Second
	}
	return &ConnectionStateService{pool: pool, activeWindow: activeWindow, slowRTTMS: 100}
}

type observedConnectionRow struct {
	SourceVMID     string
	DestVMID       string
	SourceName     string
	DestName       string
	SourceIP       string
	DestIP         string
	TenantID       string
	ProjectID      string
	Protocol       string
	Port           int
	Scope          string
	RequestCount   int64
	ErrorCount     int64
	BytesTotal     int64
	LastObservedAt time.Time
	LastErrorAt    sql.NullTime
}

type probeConnectionRow struct {
	SourceVMID    string
	DestVMID      string
	Protocol      string
	Port          int
	Success       bool
	RTTMs         float64
	ObservedAt    time.Time
	LastErrorText string
}

func (s *ConnectionStateService) List(ctx context.Context, filter model.ConnectionViewFilter) ([]model.ConnectionView, error) {
	if filter.TimeRange <= 0 {
		filter.TimeRange = defaultConnectionViewWindow
	}
	intents, err := s.listIntents(ctx, filter)
	if err != nil {
		return nil, err
	}
	configs, err := s.listConfigurations(ctx, filter)
	if err != nil {
		return nil, err
	}
	observed, err := s.listObserved(ctx, filter)
	if err != nil {
		return nil, err
	}
	probes, err := s.listProbes(ctx, filter)
	if err != nil {
		return nil, err
	}

	views := map[string]*model.ConnectionView{}
	configByKey := map[string]model.ConnectionConfiguration{}
	for _, cfg := range configs {
		configByKey[connectionStateKey(cfg.SourceVMID, cfg.DestVMID, cfg.Protocol, cfg.Port)] = cfg
	}

	for _, intent := range intents {
		key := connectionStateKey(intent.SourceVMID, intent.DestVMID, intent.Protocol, intent.Port)
		view := ensureConnectionView(views, key, intent.SourceVMID, intent.DestVMID, intent.Protocol, intent.Port)
		intentCopy := intent
		view.Intent = &intentCopy
		view.TenantID = valueOr(view.TenantID, intent.TenantID)
		view.ProjectID = valueOr(view.ProjectID, intent.ProjectID)
		view.IntendedState = intent.Status
		view.SecurityState = intent.Exposure
		view.LastChangedAt = maxTimePtr(view.LastChangedAt, intent.UpdatedAt)
	}

	for _, cfg := range configs {
		key := connectionStateKey(cfg.SourceVMID, cfg.DestVMID, cfg.Protocol, cfg.Port)
		view := ensureConnectionView(views, key, cfg.SourceVMID, cfg.DestVMID, cfg.Protocol, cfg.Port)
		cfgCopy := cfg
		view.Configuration = &cfgCopy
		view.TenantID = valueOr(view.TenantID, cfg.TenantID)
		view.ProjectID = valueOr(view.ProjectID, cfg.ProjectID)
		view.ConfigurationState = cfg.ConfigState
		view.SecurityState = valueOr(view.SecurityState, cfg.SecurityState)
		if cfg.LastSyncedAt != nil {
			view.LastChangedAt = maxTimePtr(view.LastChangedAt, *cfg.LastSyncedAt)
		}
	}

	for _, row := range observed {
		key := connectionStateKey(row.SourceVMID, row.DestVMID, row.Protocol, row.Port)
		view := ensureConnectionView(views, key, row.SourceVMID, row.DestVMID, row.Protocol, row.Port)
		view.SourceName = valueOr(view.SourceName, row.SourceName)
		view.DestName = valueOr(view.DestName, row.DestName)
		view.SourceIP = valueOr(view.SourceIP, row.SourceIP)
		view.DestIP = valueOr(view.DestIP, row.DestIP)
		view.TenantID = valueOr(view.TenantID, row.TenantID)
		view.ProjectID = valueOr(view.ProjectID, row.ProjectID)
		view.Scope = valueOr(view.Scope, row.Scope)
		observedAt := row.LastObservedAt
		view.Observed = model.ConnectionObservedState{
			SourceVMID: row.SourceVMID, DestVMID: row.DestVMID, Protocol: row.Protocol, Port: row.Port,
			Scope: row.Scope, Observed: true, Active: time.Since(row.LastObservedAt) <= s.activeWindow,
			RequestCount: row.RequestCount, ErrorCount: row.ErrorCount, BytesTotal: row.BytesTotal,
			LastObservedAt: &observedAt,
		}
		if row.LastErrorAt.Valid {
			errAt := row.LastErrorAt.Time
			view.Observed.LastErrorAt = &errAt
			view.Observed.Failed = time.Since(errAt) <= s.activeWindow
		}
		view.LastChangedAt = maxTimePtr(view.LastChangedAt, row.LastObservedAt)
	}

	for _, probe := range probes {
		key := connectionStateKey(probe.SourceVMID, probe.DestVMID, probe.Protocol, probe.Port)
		view := ensureConnectionView(views, key, probe.SourceVMID, probe.DestVMID, probe.Protocol, probe.Port)
		view.Observed.SourceVMID = probe.SourceVMID
		view.Observed.DestVMID = probe.DestVMID
		view.Observed.Protocol = probe.Protocol
		view.Observed.Port = probe.Port
		view.Observed.Observed = true
		view.Observed.Reachable = probe.Success
		view.Observed.AvgRTTMs = probe.RTTMs
		if !probe.Success {
			view.Observed.Failed = true
			view.Observed.ErrorCount++
			errAt := probe.ObservedAt
			view.Observed.LastErrorAt = &errAt
		}
		observedAt := probe.ObservedAt
		if view.Observed.LastObservedAt == nil || probe.ObservedAt.After(*view.Observed.LastObservedAt) {
			view.Observed.LastObservedAt = &observedAt
		}
		view.LastChangedAt = maxTimePtr(view.LastChangedAt, probe.ObservedAt)
	}

	result := make([]model.ConnectionView, 0, len(views))
	for _, view := range views {
		if view.Configuration == nil {
			if cfg, ok := configByKey[connectionStateKey(view.SourceVMID, view.DestVMID, view.Protocol, view.Port)]; ok {
				cfgCopy := cfg
				view.Configuration = &cfgCopy
			}
		}
		finalizeConnectionView(view, s.slowRTTMS)
		result = append(result, *view)
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i].LastChangedAt, result[j].LastChangedAt
		if left == nil || right == nil {
			return result[i].ID < result[j].ID
		}
		return left.After(*right)
	})
	return result, nil
}

func (s *ConnectionStateService) listIntents(ctx context.Context, filter model.ConnectionViewFilter) ([]model.ConnectionIntent, error) {
	query := `
		SELECT id::text, COALESCE(tenant_id, ''), COALESCE(project_id, ''), source_vm_id, dest_vm_id,
		       protocol, port, COALESCE(purpose, ''), exposure, required, status, COALESCE(created_by, ''),
		       created_at, updated_at
		FROM connection_intents WHERE 1=1`
	args := []any{}
	query, args = addConnectionFilter(query, args, "tenant_id", filter.TenantID)
	query, args = addConnectionFilter(query, args, "project_id", filter.ProjectID)
	if filter.VMID != "" {
		args = append(args, filter.VMID)
		query += fmt.Sprintf(" AND (source_vm_id = $%d OR dest_vm_id = $%d)", len(args), len(args))
	}
	query += " ORDER BY updated_at DESC LIMIT 2000"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.ConnectionIntent{}
	for rows.Next() {
		var item model.ConnectionIntent
		if err := rows.Scan(&item.ID, &item.TenantID, &item.ProjectID, &item.SourceVMID, &item.DestVMID, &item.Protocol, &item.Port, &item.Purpose, &item.Exposure, &item.Required, &item.Status, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *ConnectionStateService) listConfigurations(ctx context.Context, filter model.ConnectionViewFilter) ([]model.ConnectionConfiguration, error) {
	query := `
		SELECT id::text, COALESCE(tenant_id, ''), COALESCE(project_id, ''), COALESCE(intent_id::text, ''),
		       source_vm_id, dest_vm_id, protocol, port, COALESCE(firewall_rule_id::text, ''),
		       COALESCE(route_id::text, ''), COALESCE(network_id, ''), config_state, security_state, last_synced_at
		FROM connection_configurations WHERE 1=1`
	args := []any{}
	query, args = addConnectionFilter(query, args, "tenant_id", filter.TenantID)
	query, args = addConnectionFilter(query, args, "project_id", filter.ProjectID)
	if filter.VMID != "" {
		args = append(args, filter.VMID)
		query += fmt.Sprintf(" AND (source_vm_id = $%d OR dest_vm_id = $%d)", len(args), len(args))
	}
	query += " ORDER BY updated_at DESC LIMIT 2000"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.ConnectionConfiguration{}
	for rows.Next() {
		var item model.ConnectionConfiguration
		var syncedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.TenantID, &item.ProjectID, &item.IntentID, &item.SourceVMID, &item.DestVMID, &item.Protocol, &item.Port, &item.FirewallRuleID, &item.RouteID, &item.NetworkID, &item.ConfigState, &item.SecurityState, &syncedAt); err != nil {
			return nil, err
		}
		if syncedAt.Valid {
			item.LastSyncedAt = &syncedAt.Time
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *ConnectionStateService) listObserved(ctx context.Context, filter model.ConnectionViewFilter) ([]observedConnectionRow, error) {
	query := `
		SELECT f.src_vm_id, f.dst_vm_id, COALESCE(sv.name, ''), COALESCE(dv.name, ''),
		       host(f.src_ip), host(f.dst_ip), COALESCE(sv.tenant_id, ''), COALESCE(sv.project_id, ''),
		       f.protocol, COALESCE(f.dst_port, 0), f.scope, SUM(f.request_count), SUM(f.error_count),
		       SUM(f.bytes_sent + f.bytes_received), MAX(f.observed_at), MAX(f.last_error_at)
		FROM network_flows f
		JOIN vms sv ON sv.id = f.src_vm_id
		JOIN vms dv ON dv.id = f.dst_vm_id
		WHERE f.dst_vm_id IS NOT NULL
		  AND f.observed_at >= NOW() - $1::interval`
	args := []any{fmt.Sprintf("%f seconds", filter.TimeRange.Seconds())}
	if filter.TenantID != "" {
		args = append(args, filter.TenantID)
		query += fmt.Sprintf(" AND (sv.tenant_id = $%d OR dv.tenant_id = $%d)", len(args), len(args))
	}
	if filter.ProjectID != "" {
		args = append(args, filter.ProjectID)
		query += fmt.Sprintf(" AND (sv.project_id = $%d OR dv.project_id = $%d)", len(args), len(args))
	}
	if filter.VMID != "" {
		args = append(args, filter.VMID)
		query += fmt.Sprintf(" AND (f.src_vm_id = $%d OR f.dst_vm_id = $%d)", len(args), len(args))
	}
	query += `
		GROUP BY f.src_vm_id, f.dst_vm_id, sv.name, dv.name, f.src_ip, f.dst_ip, sv.tenant_id, sv.project_id, f.protocol, f.dst_port, f.scope
		ORDER BY MAX(f.observed_at) DESC LIMIT 5000`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []observedConnectionRow{}
	for rows.Next() {
		var row observedConnectionRow
		if err := rows.Scan(&row.SourceVMID, &row.DestVMID, &row.SourceName, &row.DestName, &row.SourceIP, &row.DestIP, &row.TenantID, &row.ProjectID, &row.Protocol, &row.Port, &row.Scope, &row.RequestCount, &row.ErrorCount, &row.BytesTotal, &row.LastObservedAt, &row.LastErrorAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *ConnectionStateService) listProbes(ctx context.Context, filter model.ConnectionViewFilter) ([]probeConnectionRow, error) {
	query := `
		SELECT p.src_vm_id, p.dst_vm_id, p.protocol, p.dst_port, p.success, p.rtt_ms, p.observed_at, COALESCE(p.error, '')
		FROM connection_probes p
		JOIN vms sv ON sv.id = p.src_vm_id
		JOIN vms dv ON dv.id = p.dst_vm_id
		WHERE p.source = 'vmlens_probe'
		  AND p.probe_type = 'connectivity_check'
		  AND p.observed_at >= NOW() - $1::interval`
	args := []any{fmt.Sprintf("%f seconds", filter.TimeRange.Seconds())}
	if filter.TenantID != "" {
		args = append(args, filter.TenantID)
		query += fmt.Sprintf(" AND (sv.tenant_id = $%d OR dv.tenant_id = $%d)", len(args), len(args))
	}
	if filter.ProjectID != "" {
		args = append(args, filter.ProjectID)
		query += fmt.Sprintf(" AND (sv.project_id = $%d OR dv.project_id = $%d)", len(args), len(args))
	}
	if filter.VMID != "" {
		args = append(args, filter.VMID)
		query += fmt.Sprintf(" AND (p.src_vm_id = $%d OR p.dst_vm_id = $%d)", len(args), len(args))
	}
	query += " ORDER BY p.observed_at DESC LIMIT 5000"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []probeConnectionRow{}
	for rows.Next() {
		var row probeConnectionRow
		if err := rows.Scan(&row.SourceVMID, &row.DestVMID, &row.Protocol, &row.Port, &row.Success, &row.RTTMs, &row.ObservedAt, &row.LastErrorText); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func ensureConnectionView(views map[string]*model.ConnectionView, key, sourceVMID, destVMID, protocol string, port int) *model.ConnectionView {
	if view, ok := views[key]; ok {
		return view
	}
	view := &model.ConnectionView{
		ID:         sourceVMID + "->" + destVMID + ":" + protocol + "/" + fmt.Sprint(port),
		SourceVMID: sourceVMID, DestVMID: destVMID, Protocol: protocol, Port: port,
		IntendedState: "not_declared", ConfigurationState: "unknown", ObservedState: "not_observed",
		ValidationState: "unknown", HealthState: "unknown", SecurityState: "unknown",
	}
	views[key] = view
	return view
}

func finalizeConnectionView(view *model.ConnectionView, slowRTTMS float64) {
	if view.Intent != nil && view.IntendedState == "" {
		view.IntendedState = view.Intent.Status
	}
	if view.Configuration != nil {
		view.ConfigurationState = valueOr(view.ConfigurationState, view.Configuration.ConfigState)
		view.SecurityState = valueOr(view.SecurityState, view.Configuration.SecurityState)
	}
	if view.Observed.Observed {
		if view.Observed.Failed || view.Observed.ErrorCount > 0 {
			view.ObservedState = "failed"
			view.ValidationState = "failed"
			view.HealthState = "failed"
		} else if view.Observed.Reachable || view.Observed.Active || view.Observed.RequestCount > 0 {
			view.ObservedState = "active"
			view.ValidationState = "connectivity_validated"
			view.HealthState = "healthy"
		} else {
			view.ObservedState = "inactive"
			view.ValidationState = "not_recently_validated"
			view.HealthState = "inactive"
		}
		if view.Observed.AvgRTTMs >= slowRTTMS {
			view.HealthState = "degraded"
			view.ValidationState = "connectivity_slow"
		}
	} else if view.Intent != nil {
		view.ObservedState = "not_observed"
		view.ValidationState = "not_validated"
		view.HealthState = "inactive"
	}
	if view.ConfigurationState == "denied" || view.ConfigurationState == "blocked" || view.IntendedState == "blocked" {
		view.HealthState = "blocked"
		view.ValidationState = "blocked_by_config"
	}
	view.Description = connectionDescription(view)
}

func connectionDescription(view *model.ConnectionView) string {
	switch {
	case view.Intent == nil && view.Observed.Observed:
		return "Active but undocumented: VMLens observed traffic/probe, but no connection intent exists yet."
	case view.Intent != nil && !view.Observed.Observed:
		return "Intended but inactive: connection exists in the model, but VMLens has not observed recent traffic or probe evidence."
	case view.Configuration == nil:
		return "Observed connectivity exists, but firewall/route configuration has not been synced from a cloud provider yet."
	case view.HealthState == "degraded":
		return "Connection is reachable, but latency is above the configured slow RTT threshold."
	case view.HealthState == "failed":
		return "Recent connection evidence contains failed attempts or probe failures."
	case view.HealthState == "blocked":
		return "Connection is blocked by intended/configured network state."
	default:
		return "Connection has model context and recent observability evidence."
	}
}

func connectionStateKey(sourceVMID, destVMID, protocol string, port int) string {
	return strings.Join([]string{sourceVMID, destVMID, protocol, fmt.Sprint(port)}, "|")
}

func addConnectionFilter(query string, args []any, column, value string) (string, []any) {
	value = strings.TrimSpace(value)
	if value == "" {
		return query, args
	}
	args = append(args, value)
	return query + fmt.Sprintf(" AND %s = $%d", column, len(args)), args
}

func maxTimePtr(current *time.Time, candidate time.Time) *time.Time {
	if current == nil || candidate.After(*current) {
		value := candidate
		return &value
	}
	return current
}
