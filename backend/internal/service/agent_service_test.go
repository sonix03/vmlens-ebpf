package service

import (
	"testing"
	"time"
)

func TestAgentStatusForLastSeen(t *testing.T) {
	now := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		lastSeen time.Time
		want     string
	}{
		{name: "current heartbeat is online", lastSeen: now, want: "online"},
		{name: "future timestamp is online", lastSeen: now.Add(time.Second), want: "online"},
		{name: "inside online window is online", lastSeen: now.Add(-59 * time.Second), want: "online"},
		{name: "online boundary is online", lastSeen: now.Add(-agentOnlineWindow), want: "online"},
		{name: "after online window is stale", lastSeen: now.Add(-agentOnlineWindow - time.Second), want: "stale"},
		{name: "stale boundary is stale", lastSeen: now.Add(-agentStaleWindow), want: "stale"},
		{name: "after stale window is offline", lastSeen: now.Add(-agentStaleWindow - time.Second), want: "offline"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := agentStatusForLastSeen(now, tt.lastSeen); got != tt.want {
				t.Fatalf("agentStatusForLastSeen() = %q, want %q", got, tt.want)
			}
		})
	}
}
