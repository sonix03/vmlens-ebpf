package service

import (
	"testing"
	"time"

	"github.com/vmlens/vmlens/backend/internal/model"
)

func TestInferRequestCount(t *testing.T) {
	if got := inferRequestCount(model.FlowEvent{Protocol: "tcp", ConnectionCount: 4}); got != 4 {
		t.Fatalf("tcp request count = %d, want 4", got)
	}
	if got := inferRequestCount(model.FlowEvent{Protocol: "udp", Direction: "egress", BytesSent: 64}); got != 1 {
		t.Fatalf("udp egress request count = %d, want 1", got)
	}
	if got := inferRequestCount(model.FlowEvent{Protocol: "icmp", Direction: "egress", BytesSent: 84}); got != 1 {
		t.Fatalf("icmp egress request count = %d, want 1", got)
	}
	if got := inferRequestCount(model.FlowEvent{Protocol: "tcp", Direction: "egress", BytesSent: 64}); got != 0 {
		t.Fatalf("tcp io request count = %d, want 0", got)
	}
}

func TestRatePerSecondUsesOneSecondMinimumWindow(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	if got := ratePerSecond(3, now, now.Add(500*time.Millisecond)); got != 3 {
		t.Fatalf("rate = %f, want 3", got)
	}
	if got := ratePerSecond(6, now, now.Add(3*time.Second)); got != 2 {
		t.Fatalf("rate = %f, want 2", got)
	}
}
