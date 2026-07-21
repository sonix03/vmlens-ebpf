package model

import "time"

type DeepFlowL4Flow struct {
	Time              time.Time `json:"time"`
	SourceIP          string    `json:"source_ip"`
	DestIP            string    `json:"dest_ip"`
	ClientPort        int       `json:"client_port"`
	ServerPort        int       `json:"server_port"`
	Protocol          string    `json:"protocol"`
	Status            string    `json:"status"`
	ByteTX            int64     `json:"byte_tx"`
	ByteRX            int64     `json:"byte_rx"`
	TotalBytes        int64     `json:"total_bytes"`
	RTTMs             float64   `json:"rtt_ms"`
	RetransTotal      int64     `json:"retrans_total"`
	AgentID           string    `json:"agent_id"`
	L3EPCID0          int       `json:"l3_epc_id_0"`
	L3EPCID1          int       `json:"l3_epc_id_1"`
	InternetDirection string    `json:"internet_direction"`
	ObservationPoint  string    `json:"observation_point,omitempty"`
}

type DeepFlowL7Request struct {
	Time               time.Time `json:"time"`
	SourceIP           string    `json:"source_ip"`
	DestIP             string    `json:"dest_ip"`
	RequestType        string    `json:"request_type"`
	RequestDomain      string    `json:"request_domain"`
	RequestResource    string    `json:"request_resource"`
	ResponseCode       int       `json:"response_code"`
	ResponseDurationMs float64   `json:"response_duration_ms"`
	RequestLength      int64     `json:"request_length"`
	ResponseLength     int64     `json:"response_length"`
	L7Protocol         string    `json:"l7_protocol_str"`
	AgentID            string    `json:"agent_id"`
	ObservationPoint   string    `json:"observation_point"`
	InternetDirection  string    `json:"internet_direction"`
	L3EPCID0           int       `json:"l3_epc_id_0,omitempty"`
	L3EPCID1           int       `json:"l3_epc_id_1,omitempty"`
}

type DeepFlowAgentMapping struct {
	AgentID       string `json:"agent_id"`
	AgentName     string `json:"agent_name"`
	VMName        string `json:"vm_name"`
	InterfaceName string `json:"interface_name"`
	TapPort       int    `json:"tap_port"`
}

type DeepFlowNode struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Label    string `json:"label"`
	IP       string `json:"ip,omitempty"`
	VMID     string `json:"vm_id,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
	Status   string `json:"status,omitempty"`
	Role     string `json:"role,omitempty"`
	Masked   bool   `json:"masked,omitempty"`
}

type DeepFlowEdge struct {
	ID                    string    `json:"id"`
	Source                string    `json:"source"`
	Target                string    `json:"target"`
	SourceVMID            string    `json:"source_vm_id,omitempty"`
	DestVMID              string    `json:"dest_vm_id,omitempty"`
	SourceIP              string    `json:"source_ip"`
	DestIP                string    `json:"dest_ip"`
	SourceRole            string    `json:"source_role"`
	DestRole              string    `json:"dest_role"`
	Direction             string    `json:"direction"`
	Protocol              string    `json:"protocol"`
	ServerPort            int       `json:"server_port"`
	RequestCount          int64     `json:"request_count"`
	ErrorCount            int64     `json:"error_count"`
	TotalBytes            int64     `json:"total_bytes"`
	AvgRTTMs              float64   `json:"avg_rtt_ms"`
	P95RTTMs              float64   `json:"p95_rtt_ms"`
	AvgResponseDurationMs float64   `json:"avg_response_duration_ms"`
	LastResponseCode      int       `json:"last_response_code,omitempty"`
	LastSeen              time.Time `json:"last_seen"`
	AgentIDs              []string  `json:"agent_ids"`
	ObservationPoints     []string  `json:"observation_points"`
}

type DeepFlowTopology struct {
	Nodes       []DeepFlowNode `json:"nodes"`
	Edges       []DeepFlowEdge `json:"edges"`
	Window      string         `json:"window"`
	GeneratedAt time.Time      `json:"generated_at"`
	Source      string         `json:"source"`
	Warnings    []string       `json:"warnings,omitempty"`
}

type DeepFlowRawLogs struct {
	L4       []DeepFlowL4Flow       `json:"l4"`
	L7       []DeepFlowL7Request    `json:"l7"`
	Mappings []DeepFlowAgentMapping `json:"mappings,omitempty"`
	Window   string                 `json:"window"`
	Limit    int                    `json:"limit"`
	Warnings []string               `json:"warnings,omitempty"`
}

type DeepFlowHealth struct {
	Enabled             bool                       `json:"enabled"`
	ClickHouseReachable bool                       `json:"clickhouse_reachable"`
	QuerierReachable    bool                       `json:"querier_reachable"`
	ControllerReachable bool                       `json:"controller_reachable"`
	AgentListNotEmpty   bool                       `json:"agent_list_not_empty"`
	LatestL4Timestamp   *time.Time                 `json:"latest_l4_timestamp,omitempty"`
	LatestL7Timestamp   *time.Time                 `json:"latest_l7_timestamp,omitempty"`
	PerVMAgentStatus    []DeepFlowPerVMAgentStatus `json:"per_vm_agent_status,omitempty"`
	Warnings            []string                   `json:"warnings,omitempty"`
	Errors              []string                   `json:"errors,omitempty"`
	CheckedAt           time.Time                  `json:"checked_at"`
}

type DeepFlowPerVMAgentStatus struct {
	VMID      string `json:"vm_id"`
	VMName    string `json:"vm_name"`
	PrivateIP string `json:"private_ip,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
	Status    string `json:"status"`
	Interface string `json:"interface,omitempty"`
	TapPort   int    `json:"tap_port,omitempty"`
}
