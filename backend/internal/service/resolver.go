package service

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type resolvedVM struct {
	ID       string
	Name     string
	TenantID string
	AgentID  string
}

func resolveVMByIP(ctx context.Context, tx pgx.Tx, ip string) (resolvedVM, bool, error) {
	var vm resolvedVM
	err := tx.QueryRow(ctx, `
		SELECT v.id, v.name, COALESCE(v.tenant_id, ''), COALESCE(v.agent_id, '')
		FROM vms v
		WHERE v.private_ip = $1::inet
		   OR v.public_ip = $1::inet
		   OR EXISTS (
		       SELECT 1 FROM vm_interfaces i
		       WHERE i.vm_id = v.id AND i.ip_address = $1::inet
		   )
		ORDER BY v.last_seen DESC
		LIMIT 1`, ip).Scan(&vm.ID, &vm.Name, &vm.TenantID, &vm.AgentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return resolvedVM{}, false, nil
	}
	if err != nil {
		return resolvedVM{}, false, err
	}
	return vm, true, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
