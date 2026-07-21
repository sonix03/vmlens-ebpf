package service

import (
	"context"
	"strings"
	"time"

	"github.com/vmlens/vmlens/backend/internal/config"
	"github.com/vmlens/vmlens/backend/internal/deepflow"
	"github.com/vmlens/vmlens/backend/internal/model"
)

type DeepFlowService struct {
	cfg    config.DeepFlowConfig
	client *deepflow.Client
	vms    *VMService
}

type DeepFlowRequest struct {
	Window          time.Duration
	Limit           int
	TenantID        string
	ProjectID       string
	VMID            string
	MaskExternalIPs *bool
}

func NewDeepFlowService(cfg config.DeepFlowConfig, vms *VMService) *DeepFlowService {
	return &DeepFlowService{cfg: cfg, client: deepflow.NewClient(cfg), vms: vms}
}

func (s *DeepFlowService) Topology(ctx context.Context, request DeepFlowRequest) (model.DeepFlowTopology, error) {
	if !s.cfg.Enabled {
		return model.DeepFlowTopology{Source: "deepflow", GeneratedAt: time.Now().UTC(), Warnings: []string{"DeepFlow integration is disabled"}}, nil
	}
	window := s.window(request.Window)
	limit := s.limit(request.Limit)
	mask := s.maskExternal(request.MaskExternalIPs)
	vms, err := s.vms.List(ctx)
	if err != nil {
		return model.DeepFlowTopology{}, err
	}
	allowedIPs := deepflow.InventoryIPs(vms, request.TenantID, request.ProjectID, request.VMID)
	if s.cfg.RequireInventoryFilter && len(allowedIPs) == 0 {
		return deepflow.NormalizeTopology(vms, nil, nil, nil, deepflow.TopologyOptions{
			Window: window, MaskExternalIPs: mask, TenantID: request.TenantID, ProjectID: request.ProjectID, VMID: request.VMID,
		}), nil
	}
	filter := deepflow.QueryFilter{Window: window, Limit: limit, AllowedIPs: allowedIPs}
	l4Rows, l4Err := s.client.QueryL4Flows(ctx, filter)
	l7Rows, l7Err := s.client.QueryL7Requests(ctx, filter)
	mappings, mappingErr := s.client.QueryAgentMappings(ctx)
	topology := deepflow.NormalizeTopology(vms, l4Rows, l7Rows, mappings, deepflow.TopologyOptions{
		Window: window, MaskExternalIPs: mask, TenantID: request.TenantID, ProjectID: request.ProjectID, VMID: request.VMID,
	})
	if l4Err != nil {
		topology.Warnings = append(topology.Warnings, "L4 query failed: "+l4Err.Error())
	}
	if l7Err != nil {
		topology.Warnings = append(topology.Warnings, "L7 query failed: "+l7Err.Error())
	}
	if l4Err != nil && l7Err != nil {
		topology.Warnings = append(topology.Warnings, "DeepFlow ClickHouse query failed; showing inventory only")
	}
	if mappingErr != nil {
		topology.Warnings = append(topology.Warnings, "agent mapping query failed: "+mappingErr.Error())
	}
	return topology, nil
}

func (s *DeepFlowService) RawLogs(ctx context.Context, request DeepFlowRequest) (model.DeepFlowRawLogs, error) {
	if !s.cfg.Enabled {
		return model.DeepFlowRawLogs{Window: s.window(request.Window).String(), Limit: s.limit(request.Limit)}, nil
	}
	window := s.window(request.Window)
	limit := s.limit(request.Limit)
	vms, err := s.vms.List(ctx)
	if err != nil {
		return model.DeepFlowRawLogs{}, err
	}
	allowedIPs := deepflow.InventoryIPs(vms, request.TenantID, request.ProjectID, request.VMID)
	if s.cfg.RequireInventoryFilter && len(allowedIPs) == 0 {
		return model.DeepFlowRawLogs{Window: window.String(), Limit: limit}, nil
	}
	filter := deepflow.QueryFilter{Window: window, Limit: limit, AllowedIPs: allowedIPs}
	l4Rows, l4Err := s.client.QueryL4Flows(ctx, filter)
	l7Rows, l7Err := s.client.QueryL7Requests(ctx, filter)
	mappings, mappingErr := s.client.QueryAgentMappings(ctx)
	warnings := []string{}
	if l4Err != nil {
		warnings = append(warnings, "L4 query failed: "+l4Err.Error())
	}
	if l7Err != nil {
		warnings = append(warnings, "L7 query failed: "+l7Err.Error())
	}
	if mappingErr != nil {
		mappings = nil
		warnings = append(warnings, "agent mapping query failed: "+mappingErr.Error())
	}
	return model.DeepFlowRawLogs{L4: l4Rows, L7: l7Rows, Mappings: mappings, Window: window.String(), Limit: limit, Warnings: warnings}, nil
}

func (s *DeepFlowService) Health(ctx context.Context, request DeepFlowRequest) model.DeepFlowHealth {
	health := model.DeepFlowHealth{Enabled: s.cfg.Enabled, CheckedAt: time.Now().UTC()}
	if !s.cfg.Enabled {
		health.Warnings = append(health.Warnings, "DeepFlow integration is disabled")
		return health
	}
	if err := s.client.PingClickHouse(ctx); err != nil {
		health.Errors = append(health.Errors, "ClickHouse unreachable: "+err.Error())
	} else {
		health.ClickHouseReachable = true
	}
	if err := s.client.PingHTTP(ctx, s.cfg.QuerierURL); err != nil {
		health.Warnings = append(health.Warnings, "DeepFlow Querier unreachable: "+err.Error())
	} else {
		health.QuerierReachable = true
	}
	if err := s.client.PingHTTP(ctx, s.cfg.ControllerURL); err != nil {
		health.Warnings = append(health.Warnings, "DeepFlow Controller unreachable: "+err.Error())
	} else {
		health.ControllerReachable = true
	}

	if health.ClickHouseReachable {
		if mappings, err := s.client.QueryAgentMappings(ctx); err != nil {
			health.Errors = append(health.Errors, "agent mapping query failed: "+err.Error())
		} else {
			health.AgentListNotEmpty = len(mappings) > 0
			if len(mappings) == 0 {
				health.Warnings = append(health.Warnings, "DeepFlow agent list is empty")
			}
			health.PerVMAgentStatus = s.perVMStatus(ctx, mappings, request)
		}
		if latest, err := s.client.LatestTimestamp(ctx, "flow_log.l4_flow_log"); err != nil {
			health.Errors = append(health.Errors, "latest L4 timestamp query failed: "+err.Error())
		} else {
			health.LatestL4Timestamp = latest
		}
		if latest, err := s.client.LatestTimestamp(ctx, "flow_log.l7_flow_log"); err != nil {
			health.Errors = append(health.Errors, "latest L7 timestamp query failed: "+err.Error())
		} else {
			health.LatestL7Timestamp = latest
		}
		if health.LatestL4Timestamp == nil && health.LatestL7Timestamp == nil {
			health.Warnings = append(health.Warnings, "no DeepFlow traffic observed")
		}
	}
	return health
}

func (s *DeepFlowService) perVMStatus(ctx context.Context, mappings []model.DeepFlowAgentMapping, request DeepFlowRequest) []model.DeepFlowPerVMAgentStatus {
	vms, err := s.vms.List(ctx)
	if err != nil {
		return nil
	}
	byName := map[string]model.DeepFlowAgentMapping{}
	for _, mapping := range mappings {
		key := strings.ToLower(strings.TrimSpace(mapping.VMName))
		if key != "" {
			byName[key] = mapping
		}
	}
	statuses := []model.DeepFlowPerVMAgentStatus{}
	for _, vm := range vms {
		if request.TenantID != "" && vm.TenantID != request.TenantID {
			continue
		}
		if request.ProjectID != "" && vm.TenantID != request.ProjectID {
			continue
		}
		if request.VMID != "" && vm.ID != request.VMID {
			continue
		}
		status := model.DeepFlowPerVMAgentStatus{VMID: vm.ID, VMName: vm.Name, PrivateIP: vm.PrivateIP, Status: "not_mapped"}
		if mapping, ok := byName[strings.ToLower(strings.TrimSpace(vm.Name))]; ok {
			status.AgentID = mapping.AgentID
			status.AgentName = mapping.AgentName
			status.Interface = mapping.InterfaceName
			status.TapPort = mapping.TapPort
			status.Status = "mapped"
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func (s *DeepFlowService) window(value time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	if s.cfg.DefaultWindow > 0 {
		return s.cfg.DefaultWindow
	}
	return 30 * time.Minute
}

func (s *DeepFlowService) limit(value int) int {
	max := s.cfg.MaxLimit
	if max <= 0 {
		max = 1000
	}
	if value <= 0 {
		return 100
	}
	if value > max {
		return max
	}
	return value
}

func (s *DeepFlowService) maskExternal(value *bool) bool {
	if value == nil {
		return s.cfg.MaskExternalIPs
	}
	return *value
}
