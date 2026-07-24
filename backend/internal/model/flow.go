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
	RequestCount    int64     `json:"request_count"`
	ErrorCount      int64     `json:"error_count"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	Interface       string    `json:"interface"`
}

type Flow struct {
	ID                string     `json:"id"`
	AgentID           string     `json:"agent_id,omitempty"`
	SrcVMID           string     `json:"src_vm_id,omitempty"`
	DstVMID           string     `json:"dst_vm_id,omitempty"`
	SrcIP             string     `json:"src_ip"`
	DstIP             string     `json:"dst_ip"`
	SrcPort           int        `json:"src_port"`
	DstPort           int        `json:"dst_port"`
	Protocol          string     `json:"protocol"`
	Direction         string     `json:"direction"`
	Scope             string     `json:"scope"`
	Service           string     `json:"service"`
	ServicePort       int        `json:"service_port"`
	BytesSent         int64      `json:"bytes_sent"`
	BytesReceived     int64      `json:"bytes_received"`
	Packets           int64      `json:"packets"`
	ConnectionCount   int64      `json:"connection_count"`
	RequestCount      int64      `json:"request_count"`
	ErrorCount        int64      `json:"error_count"`
	RequestsPerSec    float64    `json:"requests_per_second"`
	ConnectionsPerSec float64    `json:"connections_per_second"`
	FirstSeen         time.Time  `json:"first_seen"`
	LastSeen          time.Time  `json:"last_seen"`
	ObservedAt        time.Time  `json:"observed_at"`
	LastErrorAt       *time.Time `json:"last_error_at,omitempty"`
	InterfaceName     string     `json:"interface_name,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

type InternalActivity struct {
	ID                string    `json:"id"`
	ObserverVMID      string    `json:"observer_vm_id"`
	ObserverName      string    `json:"observer_name"`
	ObserverIP        string    `json:"observer_ip"`
	PeerVMID          string    `json:"peer_vm_id,omitempty"`
	PeerName          string    `json:"peer_name"`
	PeerIP            string    `json:"peer_ip"`
	SourceVMID        string    `json:"source_vm_id,omitempty"`
	SourceName        string    `json:"source_name"`
	SourceIP          string    `json:"source_ip"`
	DestinationVMID   string    `json:"destination_vm_id,omitempty"`
	DestinationName   string    `json:"destination_name"`
	DestinationIP     string    `json:"destination_ip"`
	Protocol          string    `json:"protocol"`
	Direction         string    `json:"direction"`
	Scope             string    `json:"scope"`
	Service           string    `json:"service"`
	ServicePort       int       `json:"service_port"`
	LocalPort         int       `json:"local_port"`
	PeerPort          int       `json:"peer_port"`
	BytesSent         int64     `json:"bytes_sent"`
	BytesReceived     int64     `json:"bytes_received"`
	ConnectionCount   int64     `json:"connection_count"`
	RequestCount      int64     `json:"request_count"`
	ErrorCount        int64     `json:"error_count"`
	RequestsPerSec    float64   `json:"requests_per_second"`
	ConnectionsPerSec float64   `json:"connections_per_second"`
	FirstSeen         time.Time `json:"first_seen"`
	LastSeen          time.Time `json:"last_seen"`
	ObservedAt        time.Time `json:"observed_at"`
}
