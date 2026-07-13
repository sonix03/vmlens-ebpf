package model

import "time"

type SSHSession struct {
	EventType  string     `json:"event_type"`
	Timestamp  time.Time  `json:"timestamp"`
	SessionID  string     `json:"session_id"`
	User       string     `json:"user"`
	RemoteIP   string     `json:"remote_ip,omitempty"`
	RemotePort int        `json:"remote_port,omitempty"`
	TTY        string     `json:"tty,omitempty"`
	AuthMethod string     `json:"auth_method,omitempty"`
	SSHPID     int        `json:"sshd_pid,omitempty"`
	StartTime  time.Time  `json:"start_time"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	Status     string     `json:"status"`
}
