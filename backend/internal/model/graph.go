package model

import "time"

type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

type GraphNode struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Label      string `json:"label"`
	IP         string `json:"ip,omitempty"`
	Status     string `json:"status,omitempty"`
	TenantID   string `json:"tenant_id,omitempty"`
	Role       string `json:"role,omitempty"`
	TrafficIn  int64  `json:"traffic_in"`
	TrafficOut int64  `json:"traffic_out"`
}

type GraphEdge struct {
	ID              string    `json:"id"`
	Source          string    `json:"source"`
	Target          string    `json:"target"`
	Protocol        string    `json:"protocol"`
	DstPort         int       `json:"dst_port"`
	Scope           string    `json:"scope"`
	BytesSent       int64     `json:"bytes_sent"`
	BytesReceived   int64     `json:"bytes_received"`
	Packets         int64     `json:"packets"`
	ConnectionCount int64     `json:"connection_count"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	LastObservedAt  time.Time `json:"last_observed_at"`
	Active          bool      `json:"active"`
	ActiveUntil     time.Time `json:"active_until"`
	Weight          int       `json:"weight"`
}

type GraphFilter struct {
	AgentID   string
	TenantID  string
	VMID      string
	Scope     string
	Protocol  string
	Port      int
	TimeRange time.Duration
	MinBytes  int64
	Status    string
}
