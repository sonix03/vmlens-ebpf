package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vmlens/vmlens/backend/internal/model"
)

type GraphService struct {
	pool             *pgxpool.Pool
	vmService        *VMService
	flowActiveWindow time.Duration
	visibility       GraphVisibility
}

type GraphVisibility struct {
	ExcludedPorts []int
	AllowedPorts  []int
	ExcludedIPs   []string
	IncludeIdle   bool
}

func NewGraphService(pool *pgxpool.Pool, vmService *VMService, flowActiveWindow time.Duration, visibility GraphVisibility) *GraphService {
	return &GraphService{pool: pool, vmService: vmService, flowActiveWindow: flowActiveWindow, visibility: visibility}
}

type graphFlowRow struct {
	AgentID       string
	SrcVMID       string
	DstVMID       string
	SrcIP         string
	DstIP         string
	SrcPort       int
	DstPort       int
	Protocol      string
	Direction     string
	Scope         string
	BytesSent     int64
	BytesReceived int64
	Packets       int64
	Connections   int64
	Requests      int64
	FirstSeen     time.Time
	LastSeen      time.Time
	ObservedAt    time.Time
	SrcName       string
	SrcTenant     string
	SrcPrivateIP  string
	SrcStatus     string
	SrcRole       string
	SrcAgentID    string
	DstName       string
	DstTenant     string
	DstPrivateIP  string
	DstStatus     string
	DstRole       string
	DstAgentID    string
}

func (s *GraphService) Get(ctx context.Context, filter model.GraphFilter) (model.Graph, error) {
	if filter.TimeRange <= 0 {
		filter.TimeRange = 15 * time.Minute
	}
	query := `
		SELECT COALESCE(f.agent_id, ''), COALESCE(f.src_vm_id, ''), COALESCE(f.dst_vm_id, ''),
		       host(f.src_ip), host(f.dst_ip), COALESCE(f.src_port, 0), COALESCE(f.dst_port, 0), f.protocol, f.direction, f.scope,
		       f.bytes_sent, f.bytes_received, f.packets, f.connection_count, f.request_count, f.first_seen, f.last_seen, f.observed_at,
		       COALESCE(sv.name, ''), COALESCE(sv.tenant_id, ''), COALESCE(host(sv.private_ip), ''),
		       COALESCE(sv.status, ''), COALESCE(sv.role, ''), COALESCE(sv.agent_id, ''),
		       COALESCE(dv.name, ''), COALESCE(dv.tenant_id, ''), COALESCE(host(dv.private_ip), ''),
		       COALESCE(dv.status, ''), COALESCE(dv.role, ''), COALESCE(dv.agent_id, '')
		FROM network_flows f
		LEFT JOIN vms sv ON sv.id = f.src_vm_id
		LEFT JOIN vms dv ON dv.id = f.dst_vm_id
		WHERE f.last_seen >= NOW() - $1::interval`
	args := []any{fmt.Sprintf("%f seconds", filter.TimeRange.Seconds())}
	add := func(condition string, value any) {
		args = append(args, value)
		placeholder := fmt.Sprintf("$%d", len(args))
		query += " AND " + strings.ReplaceAll(condition, "$%d", placeholder)
	}
	if filter.AgentID != "" {
		add("f.agent_id = $%d", filter.AgentID)
	}
	if filter.TenantID != "" {
		add("(sv.tenant_id = $%d OR dv.tenant_id = $%d)", filter.TenantID)
	}
	if filter.VMID != "" {
		add("(f.src_vm_id = $%d OR f.dst_vm_id = $%d)", filter.VMID)
	}
	if filter.Scope != "" {
		add("f.scope = $%d", filter.Scope)
	}
	if filter.Protocol != "" {
		add("f.protocol = $%d", filter.Protocol)
	}
	if filter.Port > 0 {
		add("f.dst_port = $%d", filter.Port)
	}
	if filter.MinBytes > 0 {
		add("(f.bytes_sent + f.bytes_received) >= $%d", filter.MinBytes)
	}
	if filter.Status != "" {
		add("(sv.status = $%d OR dv.status = $%d)", filter.Status)
	}
	query += " ORDER BY f.last_seen DESC LIMIT 5000"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return model.Graph{}, err
	}
	defer rows.Close()
	flowRows := []graphFlowRow{}
	for rows.Next() {
		var row graphFlowRow
		if err := rows.Scan(
			&row.AgentID, &row.SrcVMID, &row.DstVMID, &row.SrcIP, &row.DstIP, &row.SrcPort,
			&row.DstPort, &row.Protocol, &row.Direction, &row.Scope, &row.BytesSent, &row.BytesReceived,
			&row.Packets, &row.Connections, &row.Requests, &row.FirstSeen, &row.LastSeen, &row.ObservedAt,
			&row.SrcName, &row.SrcTenant, &row.SrcPrivateIP, &row.SrcStatus, &row.SrcRole, &row.SrcAgentID,
			&row.DstName, &row.DstTenant, &row.DstPrivateIP, &row.DstStatus, &row.DstRole, &row.DstAgentID,
		); err != nil {
			return model.Graph{}, err
		}
		flowRows = append(flowRows, row)
	}
	if err := rows.Err(); err != nil {
		return model.Graph{}, err
	}

	nodes := map[string]*model.GraphNode{}
	edges := map[string]*model.GraphEdge{}
	vms, err := s.vmService.List(ctx)
	if err != nil {
		return model.Graph{}, err
	}
	vmByID := make(map[string]model.VM, len(vms))
	now := time.Now()
	for _, vm := range vms {
		vmByID[vm.ID] = vm
		if filter.AgentID != "" && vm.AgentID != filter.AgentID {
			continue
		}
		if filter.TenantID != "" && vm.TenantID != filter.TenantID {
			continue
		}
		if filter.VMID != "" && vm.ID != filter.VMID {
			continue
		}
		if !visibleVM(vm, filter.Status, now) {
			continue
		}
		node := vmNode(vm.ID, vm.Name, vm.PrivateIP, vm.Status, vm.TenantID, vm.Role)
		nodes[node.ID] = &node
	}

	for _, row := range flowRows {
		row = normalizeGraphRow(row)
		if s.hideFlow(row) {
			continue
		}
		if row.SrcIP == row.DstIP || (row.SrcVMID != "" && row.SrcVMID == row.DstVMID) {
			continue
		}
		// A historical flow must not resurrect a registered VM that is absent
		// from the live/default node set (offline, stale, or filtered out).
		if _, registered := vmByID[row.SrcVMID]; registered {
			if _, visible := nodes[row.SrcVMID]; !visible {
				continue
			}
		}
		if _, registered := vmByID[row.DstVMID]; registered {
			if _, visible := nodes[row.DstVMID]; !visible {
				continue
			}
		}
		sourceID := row.SrcVMID
		if sourceID == "" {
			sourceID = "unknown-source-" + nodeSafe(row.SrcIP)
			if _, ok := nodes[sourceID]; !ok {
				nodeType := "unknown"
				if row.Scope == ScopeUnknownInternal || strings.HasPrefix(row.Scope, "internal_") {
					nodeType = "unknown_internal"
				}
				nodes[sourceID] = &model.GraphNode{ID: sourceID, Type: nodeType, Label: row.SrcIP, IP: row.SrcIP, Status: "unknown"}
			}
		} else if _, ok := nodes[sourceID]; !ok {
			node := vmNode(sourceID, valueOr(row.SrcName, row.SrcIP), valueOr(row.SrcPrivateIP, row.SrcIP), valueOr(row.SrcStatus, "unknown"), row.SrcTenant, row.SrcRole)
			nodes[sourceID] = &node
		}

		targetID := row.DstVMID
		if targetID != "" {
			if _, ok := nodes[targetID]; !ok {
				node := vmNode(targetID, valueOr(row.DstName, row.DstIP), valueOr(row.DstPrivateIP, row.DstIP), valueOr(row.DstStatus, "unknown"), row.DstTenant, row.DstRole)
				nodes[targetID] = &node
			}
		} else {
			nodeType, prefix := "unknown", "unknown-"
			status := "unknown"
			if row.Scope == ScopeUnknownInternal {
				nodeType, prefix = "unknown_internal", "unknown-internal-"
			}
			if row.Scope == ScopeExternalPublic || row.Scope == ScopeExternalPrivate {
				nodeType, prefix, status = "external", "external-", "external"
			}
			targetID = prefix + nodeSafe(row.DstIP)
			if _, ok := nodes[targetID]; !ok {
				nodes[targetID] = &model.GraphNode{ID: targetID, Type: nodeType, Label: row.DstIP, IP: row.DstIP, Status: status}
			}
		}

		edgeID := graphEdgeID(sourceID, targetID, row.Protocol, row.Scope)
		edge, ok := edges[edgeID]
		if !ok {
			edge = &model.GraphEdge{ID: edgeID, Source: sourceID, Target: targetID, Protocol: row.Protocol, DstPort: row.DstPort, Scope: row.Scope, FirstSeen: row.FirstSeen, LastSeen: row.LastSeen}
			edges[edgeID] = edge
		} else {
			edge.DstPort = preferredGraphPort(edge.DstPort, row.DstPort)
		}
		edge.BytesSent += row.BytesSent
		edge.BytesReceived += row.BytesReceived
		edge.Packets += row.Packets
		edge.ConnectionCount += row.Connections
		edge.RequestCount += row.Requests
		if row.FirstSeen.Before(edge.FirstSeen) {
			edge.FirstSeen = row.FirstSeen
		}
		if row.LastSeen.After(edge.LastSeen) {
			edge.LastSeen = row.LastSeen
		}
		if row.ObservedAt.After(edge.LastObservedAt) {
			edge.LastObservedAt = row.ObservedAt
		}
		edge.Weight = edgeWeight(edge.BytesSent + edge.BytesReceived)

		nodes[sourceID].TrafficOut += row.BytesSent
		nodes[sourceID].TrafficIn += row.BytesReceived
		nodes[targetID].TrafficIn += row.BytesSent
		nodes[targetID].TrafficOut += row.BytesReceived
	}

	result := model.Graph{Nodes: make([]model.GraphNode, 0, len(nodes)), Edges: make([]model.GraphEdge, 0, len(edges))}
	connectedNonVMNodes := map[string]bool{}
	for _, edge := range edges {
		setEdgeActivity(edge, now, edge.LastObservedAt, s.flowActiveWindow)
		if !s.visibility.IncludeIdle && !edge.Active {
			continue
		}
		connectedNonVMNodes[edge.Source] = true
		connectedNonVMNodes[edge.Target] = true
		result.Edges = append(result.Edges, *edge)
	}
	for _, node := range nodes {
		if node.Type != "vm" && !connectedNonVMNodes[node.ID] {
			continue
		}
		result.Nodes = append(result.Nodes, *node)
	}
	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].ID < result.Nodes[j].ID })
	sort.Slice(result.Edges, func(i, j int) bool { return result.Edges[i].ID < result.Edges[j].ID })
	return result, nil
}

func (s *GraphService) hideFlow(row graphFlowRow) bool {
	return hiddenByGraphVisibility(s.visibility, row.SrcPort, row.DstPort, row.SrcIP, row.DstIP)
}

func normalizeGraphRow(row graphFlowRow) graphFlowRow {
	if !shouldFlipServiceResponse(row.SrcPort, row.DstPort) {
		return row
	}
	row.SrcVMID, row.DstVMID = row.DstVMID, row.SrcVMID
	row.SrcIP, row.DstIP = row.DstIP, row.SrcIP
	row.SrcPort, row.DstPort = row.DstPort, row.SrcPort
	row.SrcName, row.DstName = row.DstName, row.SrcName
	row.SrcTenant, row.DstTenant = row.DstTenant, row.SrcTenant
	row.SrcPrivateIP, row.DstPrivateIP = row.DstPrivateIP, row.SrcPrivateIP
	row.SrcStatus, row.DstStatus = row.DstStatus, row.SrcStatus
	row.SrcRole, row.DstRole = row.DstRole, row.SrcRole
	row.SrcAgentID, row.DstAgentID = row.DstAgentID, row.SrcAgentID
	row.BytesSent, row.BytesReceived = row.BytesReceived, row.BytesSent
	return row
}

func graphEdgeID(sourceID, targetID, protocol, scope string) string {
	return fmt.Sprintf("%s->%s:%s/%s", sourceID, targetID, protocol, scope)
}

func preferredGraphPort(current, next int) int {
	if current == 0 {
		return next
	}
	if isEphemeralPort(current) && !isEphemeralPort(next) {
		return next
	}
	return current
}

func isEphemeralPort(port int) bool {
	return port >= 32768
}

func setEdgeActivity(edge *model.GraphEdge, now, observedAt time.Time, activeWindow time.Duration) {
	edge.ActiveUntil = observedAt.Add(activeWindow)
	edge.Active = now.Before(edge.ActiveUntil)
}

func visibleVM(vm model.VM, requestedStatus string, _ time.Time) bool {
	if requestedStatus != "" {
		return vm.Status == requestedStatus
	}
	// Disconnected VMs remain inventory nodes. A cloud lifecycle integration
	// can explicitly remove or mark a truly deleted instance.
	return vm.Status != "deleted"
}

func vmNode(id, name, ip, status, tenant, role string) model.GraphNode {
	return model.GraphNode{ID: id, Type: "vm", Label: name, IP: ip, Status: status, TenantID: tenant, Role: role}
}

func nodeSafe(value string) string {
	replacer := strings.NewReplacer(":", "_", "/", "_", "%", "_")
	return replacer.Replace(value)
}

func edgeWeight(bytes int64) int {
	switch {
	case bytes >= 100*1024*1024:
		return 5
	case bytes >= 10*1024*1024:
		return 4
	case bytes >= 1024*1024:
		return 3
	case bytes >= 100*1024:
		return 2
	default:
		return 1
	}
}
