package model

import "time"

type NetworkFlow struct {
	EventType string    `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id,omitempty"`
	PID       int       `json:"pid"`
	Process   string    `json:"process"`
	SrcIP     string    `json:"src_ip,omitempty"`
	DstIP     string    `json:"dst_ip,omitempty"`
	SrcPort   int       `json:"src_port,omitempty"`
	DstPort   int       `json:"dst_port,omitempty"`
	Protocol  string    `json:"protocol"`
	RXBytes   uint64    `json:"rx_bytes"`
	TXBytes   uint64    `json:"tx_bytes"`
}