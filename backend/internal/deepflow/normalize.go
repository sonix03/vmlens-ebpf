package deepflow

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/vmlens/vmlens/backend/internal/model"
)

const (
	roleInternalVM = "internal_vm"
	roleExternalIP = "external_ip"
	roleUnknown    = "unknown"

	directionInternalInternal = "internal_internal"
	directionInternalExternal = "internal_external"
	directionExternalInternal = "external_internal"
)

type TopologyOptions struct {
	Window          time.Duration
	MaskExternalIPs bool
	TenantID        string
	ProjectID       string
	VMID            string
}

type inventoryIndex struct {
	vms     []model.VM
	byID    map[string]model.VM
	byIP    map[string]model.VM
	allowed []string
}

func InventoryIPs(vms []model.VM, tenantID, projectID, vmID string) []string {
	return newInventoryIndex(vms, tenantID, projectID, vmID).allowed
}

func NormalizeTopology(
	vms []model.VM,
	l4Rows []model.DeepFlowL4Flow,
	l7Rows []model.DeepFlowL7Request,
	mappings []model.DeepFlowAgentMapping,
	options TopologyOptions,
) model.DeepFlowTopology {
	inventory := newInventoryIndex(vms, options.TenantID, options.ProjectID, options.VMID)
	nodes := make(map[string]model.DeepFlowNode)
	for _, vm := range inventory.vms {
		nodes[vm.ID] = model.DeepFlowNode{
			ID: vm.ID, Type: "vm", Label: vm.Name, IP: firstNonEmpty(vm.PrivateIP, vm.PublicIP),
			VMID: vm.ID, TenantID: vm.TenantID, Status: vm.Status, Role: vm.Role,
		}
	}

	dedupedL4 := dedupeL4(l4Rows)
	dedupedL7 := dedupeL7(l7Rows)
	portHints := l4PortHints(dedupedL4)
	edges := map[string]*edgeAccumulator{}

	for _, row := range dedupedL4 {
		if !inventory.allows(row.SourceIP, row.DestIP) {
			continue
		}
		sourceVM, sourceMapped := inventory.byIP[row.SourceIP]
		destVM, destMapped := inventory.byIP[row.DestIP]
		direction := topologyDirection(row.InternetDirection, row.L3EPCID0, row.L3EPCID1)
		sourceNode, sourceRole := topologyNode(nodes, row.SourceIP, sourceVM, sourceMapped, sourceRoleFor(direction, sourceMapped), options.MaskExternalIPs)
		destNode, destRole := topologyNode(nodes, row.DestIP, destVM, destMapped, destRoleFor(direction, destMapped), options.MaskExternalIPs)
		edge := ensureEdge(edges, sourceNode, destNode, sourceVMID(sourceVM, sourceMapped), sourceVMID(destVM, destMapped), row.SourceIP, row.DestIP, sourceRole, destRole, direction, row.Protocol, row.ServerPort, options.MaskExternalIPs)
		edge.totalBytes += row.TotalBytes
		edge.retransTotal += row.RetransTotal
		edge.l4Rows++
		if row.RTTMs > 0 {
			edge.rtts = append(edge.rtts, row.RTTMs)
		}
		if row.Time.After(edge.lastSeen) {
			edge.lastSeen = row.Time
		}
		edge.agentIDs.add(row.AgentID)
		edge.observationPoints.add(firstNonEmpty(row.ObservationPoint, "l4"))
		if isFlowError(row.Status) {
			edge.errorCount++
		}
	}

	for _, row := range dedupedL7 {
		if !inventory.allows(row.SourceIP, row.DestIP) {
			continue
		}
		sourceVM, sourceMapped := inventory.byIP[row.SourceIP]
		destVM, destMapped := inventory.byIP[row.DestIP]
		direction := topologyDirection(row.InternetDirection, row.L3EPCID0, row.L3EPCID1)
		sourceNode, sourceRole := topologyNode(nodes, row.SourceIP, sourceVM, sourceMapped, sourceRoleFor(direction, sourceMapped), options.MaskExternalIPs)
		destNode, destRole := topologyNode(nodes, row.DestIP, destVM, destMapped, destRoleFor(direction, destMapped), options.MaskExternalIPs)
		protocol := normalizeProtocol(row.L7Protocol)
		if protocol == "" || protocol == "http" || protocol == "https" {
			protocol = "tcp"
		}
		serverPort := portHints.serverPort(row.SourceIP, row.DestIP, protocol)
		edge := ensureEdge(edges, sourceNode, destNode, sourceVMID(sourceVM, sourceMapped), sourceVMID(destVM, destMapped), row.SourceIP, row.DestIP, sourceRole, destRole, direction, protocol, serverPort, options.MaskExternalIPs)
		edge.requestCount++
		edge.totalBytes += row.RequestLength + row.ResponseLength
		if row.ResponseCode >= 400 {
			edge.errorCount++
		}
		if row.ResponseDurationMs > 0 {
			edge.responseDurations = append(edge.responseDurations, row.ResponseDurationMs)
		}
		if row.Time.After(edge.lastSeen) {
			edge.lastSeen = row.Time
			edge.lastResponseCode = row.ResponseCode
		}
		edge.agentIDs.add(row.AgentID)
		edge.observationPoints.add(firstNonEmpty(row.ObservationPoint, "l7"))
	}

	result := model.DeepFlowTopology{
		Nodes:       make([]model.DeepFlowNode, 0, len(nodes)),
		Edges:       make([]model.DeepFlowEdge, 0, len(edges)),
		Window:      options.Window.String(),
		GeneratedAt: time.Now().UTC(),
		Source:      "deepflow",
	}
	if len(inventory.vms) == 0 {
		result.Warnings = append(result.Warnings, "no VM inventory matched the current tenant/project/vm filter")
	}
	if len(dedupedL4) == 0 && len(dedupedL7) == 0 {
		result.Warnings = append(result.Warnings, "no DeepFlow traffic observed in the selected window")
	}
	if len(mappings) == 0 {
		result.Warnings = append(result.Warnings, "DeepFlow agent mapping is empty")
	}

	for _, node := range nodes {
		result.Nodes = append(result.Nodes, node)
	}
	for _, acc := range edges {
		if acc.requestCount == 0 {
			acc.requestCount = acc.l4Rows
		}
		result.Edges = append(result.Edges, acc.model())
	}
	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].ID < result.Nodes[j].ID })
	sort.Slice(result.Edges, func(i, j int) bool {
		if result.Edges[i].LastSeen.Equal(result.Edges[j].LastSeen) {
			return result.Edges[i].ID < result.Edges[j].ID
		}
		return result.Edges[i].LastSeen.After(result.Edges[j].LastSeen)
	})
	return result
}

func newInventoryIndex(vms []model.VM, tenantID, projectID, vmID string) inventoryIndex {
	index := inventoryIndex{byID: map[string]model.VM{}, byIP: map[string]model.VM{}}
	for _, vm := range vms {
		if tenantID != "" && vm.TenantID != tenantID {
			continue
		}
		if projectID != "" && vm.TenantID != projectID {
			continue
		}
		if vmID != "" && vm.ID != vmID {
			continue
		}
		index.vms = append(index.vms, vm)
		index.byID[vm.ID] = vm
		for _, ip := range vmIPs(vm) {
			index.byIP[ip] = vm
			index.allowed = append(index.allowed, ip)
		}
	}
	sort.Slice(index.vms, func(i, j int) bool { return index.vms[i].Name < index.vms[j].Name })
	sort.Strings(index.allowed)
	return index
}

func (i inventoryIndex) allows(sourceIP, destIP string) bool {
	if len(i.allowed) == 0 {
		return false
	}
	_, sourceOK := i.byIP[sourceIP]
	_, destOK := i.byIP[destIP]
	return sourceOK || destOK
}

func vmIPs(vm model.VM) []string {
	set := map[string]struct{}{}
	add := func(ip string) {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			return
		}
		set[ip] = struct{}{}
	}
	add(vm.PrivateIP)
	add(vm.PublicIP)
	for _, iface := range vm.Interfaces {
		add(iface.IPAddress)
	}
	out := make([]string, 0, len(set))
	for ip := range set {
		out = append(out, ip)
	}
	sort.Strings(out)
	return out
}

func dedupeL4(rows []model.DeepFlowL4Flow) []model.DeepFlowL4Flow {
	selected := make(map[string]model.DeepFlowL4Flow, len(rows))
	for _, row := range rows {
		key := fmt.Sprintf("%d|%s|%s|%d|%d|%s", row.Time.UnixMilli()/100, row.SourceIP, row.DestIP, row.ClientPort, row.ServerPort, row.Protocol)
		current, ok := selected[key]
		if !ok || row.TotalBytes > current.TotalBytes || row.Time.After(current.Time) {
			selected[key] = row
		}
	}
	out := make([]model.DeepFlowL4Flow, 0, len(selected))
	for _, row := range selected {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out
}

func dedupeL7(rows []model.DeepFlowL7Request) []model.DeepFlowL7Request {
	selected := make(map[string]model.DeepFlowL7Request, len(rows))
	for _, row := range rows {
		key := fmt.Sprintf("%d|%s|%s|%s|%s|%s|%d|%d|%d|%s",
			row.Time.UnixMilli()/100, row.SourceIP, row.DestIP, row.RequestType, row.RequestDomain,
			row.RequestResource, row.ResponseCode, row.RequestLength, row.ResponseLength, row.L7Protocol)
		current, ok := selected[key]
		if !ok || observationPriority(row.ObservationPoint) > observationPriority(current.ObservationPoint) || row.Time.After(current.Time) {
			selected[key] = row
		}
	}
	out := make([]model.DeepFlowL7Request, 0, len(selected))
	for _, row := range selected {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out
}

func observationPriority(point string) int {
	switch normalizeObservationPoint(point) {
	case "s-p":
		return 4
	case "s":
		return 3
	case "c-p":
		return 2
	case "c":
		return 1
	default:
		return 0
	}
}

func topologyDirection(internetDirection string, epc0, epc1 int) string {
	value := strings.ReplaceAll(strings.TrimSpace(internetDirection), " ", "")
	switch value {
	case "0->1":
		return directionInternalExternal
	case "1->0":
		return directionExternalInternal
	default:
		if epc0 == -2 && epc1 != -2 {
			return directionExternalInternal
		}
		if epc0 != -2 && epc1 == -2 {
			return directionInternalExternal
		}
		return directionInternalInternal
	}
}

func sourceRoleFor(direction string, mapped bool) string {
	if mapped {
		return roleInternalVM
	}
	if direction == directionExternalInternal {
		return roleExternalIP
	}
	return roleUnknown
}

func destRoleFor(direction string, mapped bool) string {
	if mapped {
		return roleInternalVM
	}
	if direction == directionInternalExternal {
		return roleExternalIP
	}
	return roleUnknown
}

func topologyNode(nodes map[string]model.DeepFlowNode, ip string, vm model.VM, mapped bool, role string, maskExternal bool) (string, string) {
	if mapped {
		if _, ok := nodes[vm.ID]; !ok {
			nodes[vm.ID] = model.DeepFlowNode{
				ID: vm.ID, Type: "vm", Label: vm.Name, IP: firstNonEmpty(vm.PrivateIP, vm.PublicIP),
				VMID: vm.ID, TenantID: vm.TenantID, Status: vm.Status, Role: vm.Role,
			}
		}
		return vm.ID, roleInternalVM
	}
	nodeType := "unknown"
	prefix := "unknown-"
	label := ip
	nodeIP := ip
	masked := false
	if role == roleExternalIP {
		nodeType = "external"
		prefix = "external-"
		if maskExternal {
			label = maskedExternalLabel(ip)
			nodeIP = label
			masked = true
		}
	}
	id := prefix + nodeSafe(firstNonEmpty(nodeIP, ip))
	if _, ok := nodes[id]; !ok {
		nodes[id] = model.DeepFlowNode{ID: id, Type: nodeType, Label: label, IP: nodeIP, Status: nodeType, Masked: masked}
	}
	return id, role
}

func ensureEdge(edges map[string]*edgeAccumulator, sourceNode, destNode, sourceVMID, destVMID, sourceIP, destIP, sourceRole, destRole, direction, protocol string, serverPort int, maskExternal bool) *edgeAccumulator {
	if protocol == "" {
		protocol = "unknown"
	}
	sourceLabel := sourceIP
	destLabel := destIP
	if maskExternal && sourceRole == roleExternalIP {
		sourceLabel = maskedExternalLabel(sourceIP)
	}
	if maskExternal && destRole == roleExternalIP {
		destLabel = maskedExternalLabel(destIP)
	}
	id := fmt.Sprintf("%s->%s:%d/%s/%s", sourceNode, destNode, serverPort, protocol, direction)
	if edge, ok := edges[id]; ok {
		return edge
	}
	edge := &edgeAccumulator{
		id: id, source: sourceNode, target: destNode,
		sourceVMID: sourceVMID, destVMID: destVMID,
		sourceIP: sourceLabel, destIP: destLabel, sourceRole: sourceRole, destRole: destRole,
		direction: direction, protocol: protocol, serverPort: serverPort,
		agentIDs: set{}, observationPoints: set{},
	}
	edges[id] = edge
	return edge
}

type edgeAccumulator struct {
	id, source, target          string
	sourceVMID, destVMID        string
	sourceIP, destIP            string
	sourceRole, destRole        string
	direction, protocol         string
	serverPort                  int
	requestCount, errorCount    int64
	totalBytes, retransTotal    int64
	l4Rows                      int64
	lastResponseCode            int
	lastSeen                    time.Time
	rtts, responseDurations     []float64
	agentIDs, observationPoints set
}

func (e *edgeAccumulator) model() model.DeepFlowEdge {
	return model.DeepFlowEdge{
		ID: e.id, Source: e.source, Target: e.target, SourceVMID: e.sourceVMID, DestVMID: e.destVMID,
		SourceIP: e.sourceIP, DestIP: e.destIP, SourceRole: e.sourceRole, DestRole: e.destRole,
		Direction: e.direction, Protocol: e.protocol, ServerPort: e.serverPort,
		RequestCount: e.requestCount, ErrorCount: e.errorCount, TotalBytes: e.totalBytes,
		AvgRTTMs: avg(e.rtts), P95RTTMs: p95(e.rtts), AvgResponseDurationMs: avg(e.responseDurations),
		LastResponseCode: e.lastResponseCode, LastSeen: e.lastSeen,
		AgentIDs: e.agentIDs.values(), ObservationPoints: e.observationPoints.values(),
	}
}

type set map[string]struct{}

func (s set) add(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	s[value] = struct{}{}
}

func (s set) values() []string {
	out := make([]string, 0, len(s))
	for value := range s {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

type portHints map[string]int

func l4PortHints(rows []model.DeepFlowL4Flow) portHints {
	hints := portHints{}
	for _, row := range rows {
		protocol := normalizeProtocol(row.Protocol)
		hints[portHintKey(row.SourceIP, row.DestIP, protocol)] = row.ServerPort
	}
	return hints
}

func (h portHints) serverPort(sourceIP, destIP, protocol string) int {
	if port := h[portHintKey(sourceIP, destIP, protocol)]; port > 0 {
		return port
	}
	if protocol != "tcp" {
		if port := h[portHintKey(sourceIP, destIP, "tcp")]; port > 0 {
			return port
		}
	}
	return 0
}

func portHintKey(sourceIP, destIP, protocol string) string {
	return sourceIP + "|" + destIP + "|" + protocol
}

func sourceVMID(vm model.VM, mapped bool) string {
	if !mapped {
		return ""
	}
	return vm.ID
}

func isFlowError(status string) bool {
	value := strings.ToLower(status)
	return strings.Contains(value, "error") || strings.Contains(value, "timeout") || strings.Contains(value, "reset")
}

func avg(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func p95(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	index := int(float64(len(sorted)-1) * 0.95)
	return sorted[index]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nodeSafe(value string) string {
	value = strings.ReplaceAll(value, ":", "_")
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "%", "_")
	return value
}

func maskedExternalLabel(ip string) string {
	hash := sha1.Sum([]byte(ip))
	prefix := hex.EncodeToString(hash[:])[:8]
	if addr, err := netip.ParseAddr(ip); err == nil && addr.Is4() {
		return "external-" + prefix
	}
	return "external-" + prefix
}
