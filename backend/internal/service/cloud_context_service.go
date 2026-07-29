package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vmlens/vmlens/backend/internal/cloud"
	"github.com/vmlens/vmlens/backend/internal/model"
)

type CloudContextService struct {
	pool     *pgxpool.Pool
	provider cloud.Provider
}

func NewCloudContextService(pool *pgxpool.Pool, provider cloud.Provider) *CloudContextService {
	return &CloudContextService{pool: pool, provider: provider}
}

func (s *CloudContextService) Status(ctx context.Context) (model.CloudProviderStatus, error) {
	if s.provider == nil {
		return cloud.NewNoopProvider().Status(ctx)
	}
	return s.provider.Status(ctx)
}

func (s *CloudContextService) Context(ctx context.Context, filter model.CloudContextFilter) (model.CloudContext, error) {
	status, err := s.Status(ctx)
	if err != nil {
		return model.CloudContext{}, err
	}
	hosts, err := s.hosts(ctx, filter)
	if err != nil {
		return model.CloudContext{}, err
	}
	publicIPs, err := s.publicIPs(ctx, filter)
	if err != nil {
		return model.CloudContext{}, err
	}
	firewallRules, err := s.firewallRules(ctx, filter)
	if err != nil {
		return model.CloudContext{}, err
	}
	routes, err := s.routes(ctx, filter)
	if err != nil {
		return model.CloudContext{}, err
	}
	policies, err := s.networkPolicies(ctx, filter)
	if err != nil {
		return model.CloudContext{}, err
	}
	return model.CloudContext{
		Provider: status, Hosts: hosts, PublicIPs: publicIPs,
		FirewallRules: firewallRules, Routes: routes, NetworkPolicies: policies,
	}, nil
}

func (s *CloudContextService) hosts(ctx context.Context, filter model.CloudContextFilter) ([]model.CloudHost, error) {
	query := `
		SELECT id, COALESCE(host_id, ''), name, name, COALESCE(tenant_id, ''), COALESCE(project_id, ''),
		       COALESCE(region, ''), COALESCE(zone, ''), COALESCE(role, ''), 'virtual_machine',
		       COALESCE(environment, ''), COALESCE(owner, ''), COALESCE(host(private_ip), ''),
		       COALESCE(host(public_ip), ''), COALESCE(network_id, ''), COALESCE(subnet_id, ''),
		       status, last_seen, discovered_by = 'agent'
		FROM vms WHERE 1=1`
	args := []any{}
	query, args = addCloudFilter(query, args, "tenant_id", filter.TenantID)
	query, args = addCloudFilter(query, args, "project_id", filter.ProjectID)
	query, args = addCloudFilter(query, args, "id", filter.VMID)
	query += " ORDER BY name"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.CloudHost{}
	for rows.Next() {
		var host model.CloudHost
		if err := rows.Scan(
			&host.ID, &host.ProviderID, &host.Name, &host.Hostname, &host.TenantID,
			&host.ProjectID, &host.Region, &host.Zone, &host.Role, &host.Type,
			&host.Environment, &host.Owner, &host.PrivateIP, &host.PublicIP,
			&host.NetworkID, &host.SubnetID, &host.Status, &host.LastSeen, &host.ObservedOnly,
		); err != nil {
			return nil, err
		}
		result = append(result, host)
	}
	return result, rows.Err()
}

func (s *CloudContextService) publicIPs(ctx context.Context, filter model.CloudContextFilter) ([]model.PublicIP, error) {
	query := `
		SELECT id::text, COALESCE(tenant_id, ''), COALESCE(project_id, ''), COALESCE(vm_id, ''),
		       host(ip_address), COALESCE(provider_id, ''), status, assigned_at, last_seen_at
		FROM cloud_public_ips WHERE 1=1`
	args := []any{}
	query, args = addCloudFilter(query, args, "tenant_id", filter.TenantID)
	query, args = addCloudFilter(query, args, "project_id", filter.ProjectID)
	query, args = addCloudFilter(query, args, "vm_id", filter.VMID)
	query += " ORDER BY updated_at DESC LIMIT 1000"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.PublicIP{}
	for rows.Next() {
		var item model.PublicIP
		var assignedAt, lastSeenAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.TenantID, &item.ProjectID, &item.VMID, &item.IPAddress, &item.ProviderID, &item.Status, &assignedAt, &lastSeenAt); err != nil {
			return nil, err
		}
		if assignedAt.Valid {
			item.AssignedAt = &assignedAt.Time
		}
		if lastSeenAt.Valid {
			item.LastSeenAt = &lastSeenAt.Time
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *CloudContextService) firewallRules(ctx context.Context, filter model.CloudContextFilter) ([]model.FirewallRule, error) {
	query := `
		SELECT id::text, COALESCE(tenant_id, ''), COALESCE(project_id, ''), COALESCE(provider_id, ''),
		       COALESCE(name, ''), direction, protocol, COALESCE(port_min, 0), COALESCE(port_max, 0),
		       COALESCE(source_cidr::text, ''), COALESCE(dest_cidr::text, ''),
		       COALESCE(source_vm_id, ''), COALESCE(dest_vm_id, ''), action, scope,
		       COALESCE(description, ''), last_synced_at
		FROM cloud_firewall_rules WHERE 1=1`
	args := []any{}
	query, args = addCloudFilter(query, args, "tenant_id", filter.TenantID)
	query, args = addCloudFilter(query, args, "project_id", filter.ProjectID)
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
	result := []model.FirewallRule{}
	for rows.Next() {
		var item model.FirewallRule
		var syncedAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.ProjectID, &item.ProviderID, &item.Name,
			&item.Direction, &item.Protocol, &item.PortMin, &item.PortMax, &item.SourceCIDR,
			&item.DestCIDR, &item.SourceVMID, &item.DestVMID, &item.Action, &item.Scope,
			&item.Description, &syncedAt,
		); err != nil {
			return nil, err
		}
		if syncedAt.Valid {
			item.LastSyncedAt = &syncedAt.Time
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *CloudContextService) routes(ctx context.Context, filter model.CloudContextFilter) ([]model.NetworkRoute, error) {
	query := `
		SELECT id::text, COALESCE(tenant_id, ''), COALESCE(project_id, ''), COALESCE(provider_id, ''),
		       COALESCE(network_id, ''), COALESCE(subnet_id, ''), destination::text,
		       COALESCE(host(next_hop), ''), route_type, COALESCE(description, ''), last_synced_at
		FROM cloud_routes WHERE 1=1`
	args := []any{}
	query, args = addCloudFilter(query, args, "tenant_id", filter.TenantID)
	query, args = addCloudFilter(query, args, "project_id", filter.ProjectID)
	query += " ORDER BY updated_at DESC LIMIT 2000"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.NetworkRoute{}
	for rows.Next() {
		var item model.NetworkRoute
		var syncedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.TenantID, &item.ProjectID, &item.ProviderID, &item.NetworkID, &item.SubnetID, &item.Destination, &item.NextHop, &item.RouteType, &item.Description, &syncedAt); err != nil {
			return nil, err
		}
		if syncedAt.Valid {
			item.LastSyncedAt = &syncedAt.Time
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *CloudContextService) networkPolicies(ctx context.Context, filter model.CloudContextFilter) ([]model.NetworkPolicy, error) {
	query := `
		SELECT id::text, COALESCE(tenant_id, ''), COALESCE(project_id, ''), name, COALESCE(zone, ''),
		       COALESCE(source_vm_id, ''), COALESCE(dest_vm_id, ''), COALESCE(protocol, ''),
		       COALESCE(port, 0), action, COALESCE(description, ''), last_synced_at
		FROM cloud_network_policies WHERE 1=1`
	args := []any{}
	query, args = addCloudFilter(query, args, "tenant_id", filter.TenantID)
	query, args = addCloudFilter(query, args, "project_id", filter.ProjectID)
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
	result := []model.NetworkPolicy{}
	for rows.Next() {
		var item model.NetworkPolicy
		var syncedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.TenantID, &item.ProjectID, &item.Name, &item.Zone, &item.SourceVMID, &item.DestVMID, &item.Protocol, &item.Port, &item.Action, &item.Description, &syncedAt); err != nil {
			return nil, err
		}
		if syncedAt.Valid {
			item.LastSyncedAt = &syncedAt.Time
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func addCloudFilter(query string, args []any, column, value string) (string, []any) {
	value = strings.TrimSpace(value)
	if value == "" {
		return query, args
	}
	args = append(args, value)
	return query + fmt.Sprintf(" AND %s = $%d", column, len(args)), args
}
