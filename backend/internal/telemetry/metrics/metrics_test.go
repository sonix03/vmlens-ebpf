package metrics

import (
	"testing"
	"time"

	"github.com/vmlens/vmlens/backend/internal/model"
)

func TestInferRequestCount(t *testing.T) {
	if got := InferRequestCount(model.FlowEvent{Protocol: ProtocolTCP, ConnectionCount: 4}); got != 4 {
		t.Fatalf("tcp request count = %d, want 4", got)
	}
	if got := InferRequestCount(model.FlowEvent{Protocol: ProtocolTCP, ErrorCount: 1}); got != 0 {
		t.Fatalf("tcp error request count = %d, want 0", got)
	}
	if got := InferRequestCount(model.FlowEvent{Protocol: ProtocolUDP, Direction: DirectionEgress, BytesSent: 64}); got != 1 {
		t.Fatalf("udp egress request count = %d, want 1", got)
	}
	if got := InferRequestCount(model.FlowEvent{Protocol: ProtocolICMP, Direction: DirectionEgress, BytesSent: 84}); got != 1 {
		t.Fatalf("icmp egress request count = %d, want 1", got)
	}
	if got := InferRequestCount(model.FlowEvent{Protocol: ProtocolTCP, Direction: DirectionEgress, BytesSent: 64}); got != 0 {
		t.Fatalf("tcp io request count = %d, want 0", got)
	}
}

func TestRatePerSecondUsesOneSecondMinimumWindow(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	if got := RatePerSecond(3, now, now.Add(500*time.Millisecond)); got != 3 {
		t.Fatalf("rate = %f, want 3", got)
	}
	if got := RatePerSecond(6, now, now.Add(3*time.Second)); got != 2 {
		t.Fatalf("rate = %f, want 2", got)
	}
}

func TestValidateFlowEventNormalizesICMPPorts(t *testing.T) {
	now := time.Now().UTC()
	event := model.FlowEvent{
		AgentID: "agent-1", SrcIP: "10.20.20.130", DstIP: "10.20.20.199",
		SrcPort: 12345, DstPort: 33434, Protocol: ProtocolICMP, Direction: DirectionEgress,
		BytesSent: 84, Packets: 1, FirstSeen: now, LastSeen: now,
	}
	if err := ValidateFlowEvent(&event); err != nil {
		t.Fatal(err)
	}
	if event.SrcPort != 0 || event.DstPort != 0 {
		t.Fatalf("icmp ports = %d/%d, want 0/0", event.SrcPort, event.DstPort)
	}
	if event.RequestCount != 1 {
		t.Fatalf("icmp request count = %d, want 1", event.RequestCount)
	}
}

func TestConnectionSeverity(t *testing.T) {
	if got := ConnectionSeverity(true, 10, 0, 100); got != SeverityNormal {
		t.Fatalf("severity = %s, want normal", got)
	}
	if got := ConnectionSeverity(true, 150, 0, 100); got != SeverityWarning {
		t.Fatalf("severity = %s, want warning", got)
	}
	if got := ConnectionSeverity(false, 10, 0, 100); got != SeverityError {
		t.Fatalf("severity = %s, want error", got)
	}
}
