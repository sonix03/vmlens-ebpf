package service

import (
	"context"
	"crypto/sha256"
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

type AgentService struct {
	pool *pgxpool.Pool
	hub  *realtime.Hub
}

const (
	agentOnlineWindow = time.Minute
	agentStaleWindow  = 5 * time.Minute
)

func NewAgentService(pool *pgxpool.Pool, hub *realtime.Hub) *AgentService {
	return &AgentService{pool: pool, hub: hub}
}

func (s *AgentService) Register(ctx context.Context, registration model.AgentRegistration) (model.RegistrationResult, error) {
	registration.AgentID = strings.TrimSpace(registration.AgentID)
	registration.Hostname = strings.TrimSpace(registration.Hostname)
	if registration.AgentID == "" || registration.Hostname == "" {
		return model.RegistrationResult{}, fmt.Errorf("agent_id and hostname are required")
	}
	registration.Interfaces = normalizeInterfaces(registration)
	primaryIP := firstValidIP(registration.PrivateIPs, registration.Interfaces)
	primaryMAC := firstNonEmpty(registration.MACAddresses, interfaceMACs(registration.Interfaces))
	publicIP := ""
	if registration.PublicIP != nil {
		publicIP = strings.TrimSpace(*registration.PublicIP)
		if publicIP != "" {
			if _, err := netip.ParseAddr(publicIP); err != nil {
				return model.RegistrationResult{}, fmt.Errorf("invalid public_ip: %w", err)
			}
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.RegistrationResult{}, err
	}
	defer tx.Rollback(ctx)

	vmID, err := s.resolveIdentity(ctx, tx, registration, primaryIP, primaryMAC)
	if err != nil {
		return model.RegistrationResult{}, err
	}
	if vmID == "" {
		vmID = stableVMID(registration, primaryIP, primaryMAC)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO vms (
			id, name, tenant_id, private_ip, public_ip, mac_address,
			discovered_by, agent_id, machine_id, status, first_seen, last_seen
		) VALUES ($1, $2, $3, $4::inet, $5::inet, $6, 'agent', $7, $8, 'online', NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			tenant_id = COALESCE(EXCLUDED.tenant_id, vms.tenant_id),
			private_ip = COALESCE(EXCLUDED.private_ip, vms.private_ip),
			public_ip = COALESCE(EXCLUDED.public_ip, vms.public_ip),
			mac_address = COALESCE(EXCLUDED.mac_address, vms.mac_address),
			agent_id = EXCLUDED.agent_id,
			machine_id = COALESCE(EXCLUDED.machine_id, vms.machine_id),
			status = 'online', last_seen = NOW()`,
		vmID, registration.Hostname, nullIfEmpty(registration.TenantID), nullIfEmpty(primaryIP),
		nullIfEmpty(publicIP), nullIfEmpty(primaryMAC), registration.AgentID, nullIfEmpty(registration.MachineID))
	if err != nil {
		return model.RegistrationResult{}, fmt.Errorf("upsert VM: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO agents (id, vm_id, hostname, machine_id, os, kernel, agent_version, environment, status, first_seen, last_seen)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'online', NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			vm_id = EXCLUDED.vm_id, hostname = EXCLUDED.hostname,
			machine_id = COALESCE(EXCLUDED.machine_id, agents.machine_id),
			os = EXCLUDED.os, kernel = EXCLUDED.kernel,
			agent_version = EXCLUDED.agent_version, environment = EXCLUDED.environment,
			status = 'online', last_seen = NOW()`,
		registration.AgentID, vmID, registration.Hostname, nullIfEmpty(registration.MachineID),
		nullIfEmpty(registration.OS), nullIfEmpty(registration.Kernel), nullIfEmpty(registration.AgentVersion), nullIfEmpty(registration.Environment))
	if err != nil {
		return model.RegistrationResult{}, fmt.Errorf("upsert agent: %w", err)
	}

	for _, iface := range registration.Interfaces {
		if iface.Name == "" {
			continue
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO vm_interfaces (vm_id, interface_name, ip_address, mac_address)
			VALUES ($1, $2, $3::inet, $4)
			ON CONFLICT (vm_id, interface_name, ip_address, mac_address)
			DO UPDATE SET updated_at = NOW()`,
			vmID, iface.Name, nullIfEmpty(iface.IPAddress), nullIfEmpty(iface.MACAddress))
		if err != nil {
			return model.RegistrationResult{}, fmt.Errorf("upsert interface %s: %w", iface.Name, err)
		}
		if iface.IPAddress != "" {
			if err := s.resolveUnknownHost(ctx, tx, vmID, registration.TenantID, iface.IPAddress); err != nil {
				return model.RegistrationResult{}, err
			}
		}
	}
	if primaryIP != "" {
		if err := s.resolveUnknownHost(ctx, tx, vmID, registration.TenantID, primaryIP); err != nil {
			return model.RegistrationResult{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return model.RegistrationResult{}, err
	}
	result := model.RegistrationResult{AgentID: registration.AgentID, VMID: vmID, Status: "online"}
	s.hub.Broadcast("agent.registered", result)
	return result, nil
}

func (s *AgentService) resolveIdentity(ctx context.Context, tx pgx.Tx, registration model.AgentRegistration, primaryIP, primaryMAC string) (string, error) {
	queries := make([]struct {
		sql string
		arg any
	}, 0, 4)
	if registration.MachineID != "" {
		queries = append(queries, struct {
			sql string
			arg any
		}{`SELECT id FROM vms WHERE machine_id = $1 ORDER BY last_seen DESC LIMIT 1`, registration.MachineID})
	}
	queries = append(queries, struct {
		sql string
		arg any
	}{`SELECT vm_id FROM agents WHERE id = $1 AND vm_id IS NOT NULL`, registration.AgentID})
	if primaryMAC != "" {
		queries = append(queries, struct {
			sql string
			arg any
		}{`SELECT vm_id FROM vm_interfaces WHERE mac_address = $1 ORDER BY updated_at DESC LIMIT 1`, primaryMAC})
	}
	if primaryIP != "" {
		queries = append(queries, struct {
			sql string
			arg any
		}{`
			SELECT id FROM vms WHERE name = $1 AND private_ip = $2::inet ORDER BY last_seen DESC LIMIT 1`, []any{registration.Hostname, primaryIP}})
	}
	for _, query := range queries {
		var id string
		var err error
		if args, ok := query.arg.([]any); ok {
			err = tx.QueryRow(ctx, query.sql, args...).Scan(&id)
		} else {
			err = tx.QueryRow(ctx, query.sql, query.arg).Scan(&id)
		}
		if err == nil {
			return id, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
	}
	return "", nil
}

func (s *AgentService) resolveUnknownHost(ctx context.Context, tx pgx.Tx, vmID, tenantID, ip string) error {
	if _, err := tx.Exec(ctx, `UPDATE unknown_internal_hosts SET resolved_vm_id = $1, last_seen = NOW() WHERE ip = $2::inet`, vmID, ip); err != nil {
		return fmt.Errorf("resolve unknown host: %w", err)
	}
	_, err := tx.Exec(ctx, `
		UPDATE network_flows f SET
			dst_vm_id = $1,
			scope = CASE WHEN COALESCE(src.tenant_id, '') = $2 THEN 'internal_same_tenant' ELSE 'internal_cross_tenant' END
		FROM vms src
		WHERE f.src_vm_id = src.id AND f.dst_vm_id IS NULL AND f.dst_ip = $3::inet`, vmID, tenantID, ip)
	return err
}

func (s *AgentService) Heartbeat(ctx context.Context, heartbeat model.AgentHeartbeat) error {
	if heartbeat.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	seenAt := time.Now().UTC()
	tag, err := s.pool.Exec(ctx, `
		UPDATE agents SET status = 'online', last_seen = $2 WHERE id = $1`, heartbeat.AgentID, seenAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agent %q is not registered", heartbeat.AgentID)
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE vms SET status = 'online', last_seen = $2 WHERE agent_id = $1`, heartbeat.AgentID, seenAt)
	if err == nil {
		s.hub.Broadcast("agent.heartbeat", heartbeat)
	}
	return err
}

func (s *AgentService) List(ctx context.Context) ([]model.Agent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(vm_id, ''), hostname, COALESCE(machine_id, ''),
		       COALESCE(os, ''), COALESCE(kernel, ''), COALESCE(agent_version, ''),
		       COALESCE(environment, ''), status, first_seen, last_seen
		FROM agents ORDER BY last_seen DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	agents := []model.Agent{}
	for rows.Next() {
		var agent model.Agent
		if err := rows.Scan(&agent.ID, &agent.VMID, &agent.Hostname, &agent.MachineID, &agent.OS, &agent.Kernel, &agent.AgentVersion, &agent.Environment, &agent.Status, &agent.FirstSeen, &agent.LastSeen); err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

func (s *AgentService) UpdateStatuses(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	onlineWindow := fmt.Sprintf("%f seconds", agentOnlineWindow.Seconds())
	staleWindow := fmt.Sprintf("%f seconds", agentStaleWindow.Seconds())
	agentTag, err := tx.Exec(ctx, `
		UPDATE agents SET status = CASE
			WHEN last_seen >= NOW() - $1::interval THEN 'online'
			WHEN last_seen >= NOW() - $2::interval THEN 'stale'
			ELSE 'offline' END
		WHERE status IS DISTINCT FROM CASE
			WHEN last_seen >= NOW() - $1::interval THEN 'online'
			WHEN last_seen >= NOW() - $2::interval THEN 'stale'
			ELSE 'offline' END`, onlineWindow, staleWindow)
	if err != nil {
		return err
	}
	vmTag, err := tx.Exec(ctx, `
		UPDATE vms SET status = CASE
			WHEN last_seen >= NOW() - $1::interval THEN 'online'
			WHEN last_seen >= NOW() - $2::interval THEN 'stale'
			ELSE 'offline' END
		WHERE status IS DISTINCT FROM CASE
			WHEN last_seen >= NOW() - $1::interval THEN 'online'
			WHEN last_seen >= NOW() - $2::interval THEN 'stale'
			ELSE 'offline' END`, onlineWindow, staleWindow)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	updated := agentTag.RowsAffected() + vmTag.RowsAffected()
	if updated > 0 {
		s.hub.Broadcast("status.changed", map[string]any{"updated": updated})
	}
	return nil
}

func agentStatusForLastSeen(now, lastSeen time.Time) string {
	age := now.Sub(lastSeen)
	if age <= agentOnlineWindow {
		return "online"
	}
	if age <= agentStaleWindow {
		return "stale"
	}
	return "offline"
}

// DeleteExpired removes agent-backed VM nodes only after they have missed
// heartbeats for the configured retention window. The delay is intentionally
// longer than the five-minute offline threshold because heartbeat loss cannot
// prove that a cloud VM was actually deleted.
func (s *AgentService) DeleteExpired(ctx context.Context, deleteAfter time.Duration) (int64, error) {
	if deleteAfter <= 0 {
		return 0, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT v.id
		FROM vms v
		JOIN agents a ON a.vm_id = v.id
		WHERE v.discovered_by = 'agent'
		  AND GREATEST(v.last_seen, a.last_seen) < NOW() - $1::interval
		FOR UPDATE OF v, a SKIP LOCKED`, fmt.Sprintf("%f seconds", deleteAfter.Seconds()))
	if err != nil {
		return 0, err
	}
	var vmIDs []string
	seenVMs := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		if _, seen := seenVMs[id]; !seen {
			seenVMs[id] = struct{}{}
			vmIDs = append(vmIDs, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	if len(vmIDs) == 0 {
		if err := tx.Commit(ctx); err != nil {
			return 0, err
		}
		return 0, nil
	}
	if _, err := tx.Exec(ctx, `DELETE FROM agents WHERE vm_id = ANY($1)`, vmIDs); err != nil {
		return 0, fmt.Errorf("delete expired agents: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM vms WHERE id = ANY($1)`, vmIDs); err != nil {
		return 0, fmt.Errorf("delete expired VMs: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	s.hub.Broadcast("vm.deleted", map[string]any{"vm_ids": vmIDs})
	return int64(len(vmIDs)), nil
}

func stableVMID(reg model.AgentRegistration, ip, mac string) string {
	identity := firstNonEmpty([]string{reg.MachineID, reg.AgentID, mac, reg.Hostname + "|" + ip})
	sum := sha256.Sum256([]byte(identity))
	return fmt.Sprintf("vm-%x", sum[:8])
}

func normalizeInterfaces(reg model.AgentRegistration) []model.Interface {
	if len(reg.Interfaces) > 0 {
		return reg.Interfaces
	}
	out := make([]model.Interface, 0, len(reg.PrivateIPs))
	for i, ip := range reg.PrivateIPs {
		iface := model.Interface{Name: fmt.Sprintf("interface-%d", i), IPAddress: ip}
		if i < len(reg.MACAddresses) {
			iface.MACAddress = reg.MACAddresses[i]
		}
		out = append(out, iface)
	}
	return out
}

func firstValidIP(ips []string, interfaces []model.Interface) string {
	for _, ip := range append(append([]string{}, ips...), interfaceIPs(interfaces)...) {
		if addr, err := netip.ParseAddr(strings.TrimSpace(ip)); err == nil && addr.IsValid() && !addr.IsLoopback() {
			return addr.String()
		}
	}
	return ""
}

func interfaceIPs(interfaces []model.Interface) []string {
	out := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		out = append(out, iface.IPAddress)
	}
	return out
}

func interfaceMACs(interfaces []model.Interface) []string {
	out := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		out = append(out, iface.MACAddress)
	}
	return out
}

func firstNonEmpty(groups ...[]string) string {
	for _, group := range groups {
		for _, value := range group {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	return ""
}
