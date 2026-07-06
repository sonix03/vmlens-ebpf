package model

import "time"

type Summary struct {
	TotalVMs        int       `json:"total_vms"`
	OnlineVMs       int       `json:"online_vms"`
	StaleVMs        int       `json:"stale_vms"`
	OfflineVMs      int       `json:"offline_vms"`
	TotalFlows      int       `json:"total_flows"`
	InternalFlows   int       `json:"internal_flows"`
	ExternalFlows   int       `json:"external_flows"`
	InternalBytes   int64     `json:"internal_bytes"`
	InternalSent    int64     `json:"internal_sent_bytes"`
	InternalRecv    int64     `json:"internal_received_bytes"`
	ExternalBytes   int64     `json:"external_bytes"`
	ExternalSent    int64     `json:"external_sent_bytes"`
	ExternalRecv    int64     `json:"external_received_bytes"`
	UnknownInternal int       `json:"unknown_internal_hosts"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type TopTalker struct {
	VMID          string `json:"vm_id"`
	Name          string `json:"name"`
	BytesSent     int64  `json:"bytes_sent"`
	BytesReceived int64  `json:"bytes_received"`
	TotalBytes    int64  `json:"total_bytes"`
}
