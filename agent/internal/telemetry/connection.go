package telemetry

import "time"

type ConnectionProbeTarget struct {
	SourceVMID string    `json:"source_vm_id"`
	SourceIP   string    `json:"source_ip"`
	DestVMID   string    `json:"dest_vm_id"`
	DestName   string    `json:"dest_name"`
	DestIP     string    `json:"dest_ip"`
	Protocol   string    `json:"protocol"`
	DestPort   int       `json:"dest_port"`
	LastSeen   time.Time `json:"last_seen"`
}

type ConnectionProbeEvent struct {
	AgentID              string    `json:"agent_id"`
	SourceIP             string    `json:"source_ip,omitempty"`
	DestIP               string    `json:"dest_ip"`
	Protocol             string    `json:"protocol"`
	DestPort             int       `json:"dest_port"`
	Success              bool      `json:"success"`
	RTTMs                float64   `json:"rtt_ms,omitempty"`
	Error                string    `json:"error,omitempty"`
	Source               string    `json:"source"`
	Type                 string    `json:"type"`
	CountedAsRequest     bool      `json:"counted_as_request"`
	CountedAsUserTraffic bool      `json:"counted_as_user_traffic"`
	Timestamp            time.Time `json:"timestamp"`
}
