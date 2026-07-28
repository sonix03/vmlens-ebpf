package metrics

import "github.com/vmlens/vmlens/agent/internal/telemetry"

func ApplyDirectionalBytes(event *telemetry.FlowEvent, bytes int64) {
	if event == nil || bytes <= 0 {
		return
	}
	if event.Direction == DirectionIngress {
		event.BytesReceived += bytes
		return
	}
	event.BytesSent += bytes
}
