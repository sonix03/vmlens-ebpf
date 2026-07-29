package connection

import (
	"testing"

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
	var egress flowbytes.Model
	flowbytes.Apply(&egress, direction.Egress, 42)
	if egress.Sent != 42 || egress.Received != 0 {
		t.Fatalf("egress bytes = sent %d received %d", egress.Sent, egress.Received)
	}

	var ingress flowbytes.Model
	flowbytes.Apply(&ingress, direction.Ingress, 55)
	if ingress.Received != 55 || ingress.Sent != 0 {
		t.Fatalf("ingress bytes = sent %d received %d", ingress.Sent, ingress.Received)
	}
}

func TestNormalizePorts(t *testing.T) {
	src, dst := classification.NormalizePorts(classification.ProtocolICMP, 12345, 33434)
	if src != 0 || dst != 0 {
		t.Fatalf("icmp ports = %d/%d, want 0/0", src, dst)
	}
}
