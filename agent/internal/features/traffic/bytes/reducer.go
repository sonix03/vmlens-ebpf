package bytes

import (
	telemetry "github.com/vmlens/vmlens/agent/internal/exporter"
	"github.com/vmlens/vmlens/agent/internal/features/traffic/direction"
)

func ApplyDirectionalBytes(event *telemetry.FlowEvent, bytes int64) {
	if event == nil || bytes <= 0 {
		return
	}
	if event.Direction == direction.Ingress {
		event.BytesReceived += bytes
		return
	}
	event.BytesSent += bytes
}
