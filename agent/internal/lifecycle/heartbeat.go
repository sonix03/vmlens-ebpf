package lifecycle

import (
	"context"
	"log"
	"time"

	"github.com/vmlens/vmlens/agent/internal/telemetry"
	"github.com/vmlens/vmlens/agent/internal/transport"
)

func Run(ctx context.Context, registration telemetry.Registration, interval time.Duration, client *transport.Sender) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			heartbeat := telemetry.Heartbeat{AgentID: registration.AgentID, Status: "online", Timestamp: now.UTC().Format(time.RFC3339Nano)}
			if err := client.Heartbeat(ctx, heartbeat); err != nil {
				log.Printf("heartbeat: %v", err)
				// Registration is idempotent. Retrying it here lets a live VM recover
				// if its node was cleaned up after an extended network partition.
				if result, registerErr := client.Register(ctx, registration); registerErr != nil {
					log.Printf("heartbeat recovery registration: %v", registerErr)
				} else {
					log.Printf("heartbeat recovery registered agent=%s vm=%s", result.AgentID, result.VMID)
				}
			}
		}
	}
}
