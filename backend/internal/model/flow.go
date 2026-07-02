package model

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
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	Interface       string    `json:"interface"`
}

type Flow struct {
	ID              string    `json:"id"`
	AgentID         string    `json:"agent_id,omitempty"`
	SrcVMID         string    `json:"src_vm_id,omitempty"`
	DstVMID         string    `json:"dst_vm_id,omitempty"`
	SrcIP           string    `json:"src_ip"`
	DstIP           string    `json:"dst_ip"`
	SrcPort         int       `json:"src_port"`
	DstPort         int       `json:"dst_port"`
	Protocol        string    `json:"protocol"`
	Direction       string    `json:"direction"`
	Scope           string    `json:"scope"`
	BytesSent       int64     `json:"bytes_sent"`
	BytesReceived   int64     `json:"bytes_received"`
	Packets         int64     `json:"packets"`
	ConnectionCount int64     `json:"connection_count"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	InterfaceName   string    `json:"interface_name,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}
