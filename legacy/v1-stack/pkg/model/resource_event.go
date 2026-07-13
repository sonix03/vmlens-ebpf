package model

import "time"

type ResourceEvent struct {
	EventType      string    `json:"event_type"`
	Timestamp      time.Time `json:"timestamp"`
	SessionID      string    `json:"session_id,omitempty"`
	PID            int       `json:"pid"`
	Process        string    `json:"process"`
	CPUPercent     float64   `json:"cpu_percent"`
	RSSBytes       uint64    `json:"rss_bytes"`
	DiskReadBytes  uint64    `json:"disk_read_bytes"`
	DiskWriteBytes uint64    `json:"disk_write_bytes"`
}

type AnalysisSummary struct {
	EventType  string    `json:"event_type"`
	Timestamp  time.Time `json:"timestamp"`
	Severity   string    `json:"severity"`
	Reason     string    `json:"reason"`
	SessionID  string    `json:"session_id,omitempty"`
	User       string    `json:"user,omitempty"`
	RemoteIP   string    `json:"remote_ip,omitempty"`
	TopProcess string    `json:"top_process"`
	PID        int       `json:"pid"`
	Summary    string    `json:"summary"`
}
