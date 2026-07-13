package model

import "time"

type ProcessEvent struct {
	EventType     string     `json:"event_type"`
	Timestamp     time.Time  `json:"timestamp"`
	SessionID     string     `json:"session_id,omitempty"`
	PID           int        `json:"pid"`
	PPID          int        `json:"ppid"`
	UID           uint32     `json:"uid"`
	GID           uint32     `json:"gid"`
	User          string     `json:"user,omitempty"`
	TTY           string     `json:"tty,omitempty"`
	Process       string     `json:"process"`
	Executable    string     `json:"executable,omitempty"`
	Command       string     `json:"command,omitempty"`
	ArgvSanitized []string   `json:"argv_sanitized,omitempty"`
	StartTime     time.Time  `json:"start_time,omitempty"`
	EndTime       *time.Time `json:"end_time,omitempty"`
	ExitCode      *int       `json:"exit_code,omitempty"`
}
