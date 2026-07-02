package heartbeat

import (
	"context"
	"log"
	"time"

	"github.com/vmlens/vmlens/agent/internal/model"
	"github.com/vmlens/vmlens/agent/internal/sender"
)

func Run(ctx context.Context, agentID string, interval time.Duration, client *sender.Sender) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			heartbeat := model.Heartbeat{AgentID: agentID, Status: "online", Timestamp: now.UTC().Format(time.RFC3339Nano)}
			if err := client.Heartbeat(ctx, heartbeat); err != nil {
				log.Printf("heartbeat: %v", err)
			}
		}
	}
}
