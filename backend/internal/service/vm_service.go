package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vmlens/vmlens/backend/internal/model"
)

type VMService struct{ pool *pgxpool.Pool }

func NewVMService(pool *pgxpool.Pool) *VMService { return &VMService{pool: pool} }

func (s *VMService) List(ctx context.Context) ([]model.VM, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, COALESCE(tenant_id, ''), COALESCE(private_ip::text, ''),
		       COALESCE(public_ip::text, ''), COALESCE(mac_address, ''), COALESCE(host_id, ''),
		       COALESCE(role, ''), discovered_by, COALESCE(agent_id, ''), COALESCE(machine_id, ''),
		       status, first_seen, last_seen, created_at
		FROM vms ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	vms := []model.VM{}
	byID := map[string]int{}
	for rows.Next() {
		var vm model.VM
		if err := rows.Scan(&vm.ID, &vm.Name, &vm.TenantID, &vm.PrivateIP, &vm.PublicIP, &vm.MACAddress, &vm.HostID, &vm.Role, &vm.DiscoveredBy, &vm.AgentID, &vm.MachineID, &vm.Status, &vm.FirstSeen, &vm.LastSeen, &vm.CreatedAt); err != nil {
			return nil, err
		}
		byID[vm.ID] = len(vms)
		vms = append(vms, vm)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ifaceRows, err := s.pool.Query(ctx, `SELECT vm_id, interface_name, COALESCE(ip_address::text, ''), COALESCE(mac_address, '') FROM vm_interfaces ORDER BY interface_name`)
	if err != nil {
		return nil, err
	}
	defer ifaceRows.Close()
	for ifaceRows.Next() {
		var vmID string
		var iface model.Interface
		if err := ifaceRows.Scan(&vmID, &iface.Name, &iface.IPAddress, &iface.MACAddress); err != nil {
			return nil, err
		}
		if index, ok := byID[vmID]; ok {
			vms[index].Interfaces = append(vms[index].Interfaces, iface)
		}
	}
	return vms, ifaceRows.Err()
}
