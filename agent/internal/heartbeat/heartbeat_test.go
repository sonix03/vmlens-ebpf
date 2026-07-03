package heartbeat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vmlens/vmlens/agent/internal/model"
	"github.com/vmlens/vmlens/agent/internal/sender"
)

func TestRunRegistersAgainAfterHeartbeatFailure(t *testing.T) {
	recovered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/agents/heartbeat":
			http.Error(w, "agent is not registered", http.StatusBadRequest)
		case "/api/agents/register":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"agent_id":"agent-test","vm_id":"vm-test","status":"online"}`))
			select {
			case recovered <- struct{}{}:
			default:
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	registration := model.Registration{AgentID: "agent-test", Hostname: "vm-test", AgentVersion: "test"}
	go Run(ctx, registration, time.Millisecond, sender.New(server.URL, time.Second))

	select {
	case <-recovered:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("agent did not recover registration")
	}
}
