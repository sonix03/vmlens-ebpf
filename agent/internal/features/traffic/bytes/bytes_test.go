package bytes

import (
	"testing"

	telemetry "github.com/vmlens/vmlens/agent/internal/exporter"
	"github.com/vmlens/vmlens/agent/internal/features/traffic/direction"
)

func TestApplyDirectionalBytes(t *testing.T) {
	event := telemetry.FlowEvent{Direction: direction.Egress}
	ApplyDirectionalBytes(&event, 12)
	if event.BytesSent != 12 || event.BytesReceived != 0 {
		t.Fatalf("unexpected egress bytes: sent=%d received=%d", event.BytesSent, event.BytesReceived)
	}

	event.Direction = direction.Ingress
	ApplyDirectionalBytes(&event, 8)
	if event.BytesSent != 12 || event.BytesReceived != 8 {
		t.Fatalf("unexpected ingress bytes: sent=%d received=%d", event.BytesSent, event.BytesReceived)
	}
}
