package capture

import (
	"net"
	"testing"

	"github.com/vmlens/vmlens/agent/internal/metrics"
	"github.com/vmlens/vmlens/agent/internal/telemetry"
)

func TestSocketIPSupportsIPv4AndIPv6(t *testing.T) {
	var ipv4 [16]byte
	copy(ipv4[:], net.ParseIP("10.20.20.130").To4())
	if got := socketIP(ipv4, 2); got != "10.20.20.130" {
		t.Fatalf("IPv4 = %s", got)
	}

	var ipv6 [16]byte
	copy(ipv6[:], net.ParseIP("fd00::130").To16())
	if got := socketIP(ipv6, 10); got != "fd00::130" {
		t.Fatalf("IPv6 = %s", got)
	}
}

func TestIPv6FallbackUsesRegisteredInterface(t *testing.T) {
	collector := EBPFCollector{registration: telemetry.Registration{Interfaces: []telemetry.Interface{
		{Name: "ens3", IPAddress: "10.20.20.130"},
		{Name: "ens3", IPAddress: "fd00::130"},
	}}}
	if got := collector.fallbackSource(10); got != "fd00::130" {
		t.Fatalf("fallback = %s", got)
	}
}

func TestRequestCountUsesConnectionsAndUDPMessages(t *testing.T) {
	if got := metrics.InferRequestCount("tcp", "egress", 0, 3, 0); got != 3 {
		t.Fatalf("tcp request count = %d, want 3", got)
	}
	if got := metrics.InferRequestCount("tcp", "egress", 0, 0, 1); got != 0 {
		t.Fatalf("tcp rst request count = %d, want 0", got)
	}
	if got := metrics.InferRequestCount("udp", "egress", 128, 0, 0); got != 1 {
		t.Fatalf("udp request count = %d, want 1", got)
	}
	if got := metrics.InferRequestCount("icmp", "egress", 84, 0, 0); got != 1 {
		t.Fatalf("icmp request count = %d, want 1", got)
	}
	if got := metrics.InferRequestCount("tcp", "egress", 128, 0, 0); got != 0 {
		t.Fatalf("tcp io request count = %d, want 0", got)
	}
}
