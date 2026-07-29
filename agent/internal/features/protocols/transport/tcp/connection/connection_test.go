package connection

import (
	"testing"

	telemetry "github.com/vmlens/vmlens/agent/internal/exporter"
	"github.com/vmlens/vmlens/agent/internal/features/classification"
	flowbytes "github.com/vmlens/vmlens/agent/internal/features/traffic/bytes"
	"github.com/vmlens/vmlens/agent/internal/features/traffic/direction"
)

func TestInferRequestCount(t *testing.T) {
	if got := InferRequestCount(classification.ProtocolTCP, direction.Egress, 0, 2, 0); got != 2 {
		t.Fatalf("tcp connection request count = %d, want 2", got)
	}
	if got := InferRequestCount(classification.ProtocolTCP, direction.Egress, 0, 0, 1); got != 0 {
		t.Fatalf("tcp failed request count = %d, want 0", got)
	}
	if got := InferRequestCount(classification.ProtocolUDP, direction.Egress, 64, 0, 0); got != 1 {
		t.Fatalf("udp request count = %d, want 1", got)
	}
	if got := InferRequestCount(classification.ProtocolICMP, direction.Ingress, 84, 0, 0); got != 1 {
		t.Fatalf("icmp request count = %d, want 1", got)
	}
}

func TestApplyDirectionalBytes(t *testing.T) {
	egress := telemetry.FlowEvent{Direction: direction.Egress}
	flowbytes.ApplyDirectionalBytes(&egress, 42)
	if egress.BytesSent != 42 || egress.BytesReceived != 0 {
		t.Fatalf("egress bytes = sent %d received %d", egress.BytesSent, egress.BytesReceived)
	}

	ingress := telemetry.FlowEvent{Direction: direction.Ingress}
	flowbytes.ApplyDirectionalBytes(&ingress, 55)
	if ingress.BytesReceived != 55 || ingress.BytesSent != 0 {
		t.Fatalf("ingress bytes = sent %d received %d", ingress.BytesSent, ingress.BytesReceived)
	}
}

func TestNormalizePorts(t *testing.T) {
	src, dst := classification.NormalizePorts(classification.ProtocolICMP, 12345, 33434)
	if src != 0 || dst != 0 {
		t.Fatalf("icmp ports = %d/%d, want 0/0", src, dst)
	}
}
