package telemetry

import "time"

type FlowEvent struct {
	AgentID         string    `json:"agent_id"`
	SrcIP           string    `json:"src_ip"`
	DstIP           string    `json:"dst_ip"`
	SrcPort         int       `json:"src_port"`
	DstPort         int       `json:"dst_port"`
	Protocol        string    `json:"protocol"`
	Direction       string    `json:"direction"`
	BytesSent       int64     `json:"bytes_sent"`
	BytesReceived   int64     `json:"bytes_received"`
	Packets         int64     `json:"packets"`
	ConnectionCount int64     `json:"connection_count"`
	RequestCount    int64     `json:"request_count"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	Interface       string    `json:"interface"`
}
