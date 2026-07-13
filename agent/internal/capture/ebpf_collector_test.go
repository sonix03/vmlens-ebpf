package capture

import (
	"net"
	"testing"

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
	if got := requestCount("tcp", "egress", rawFlowEvent{Connections: 3}); got != 3 {
		t.Fatalf("tcp request count = %d, want 3", got)
	}
	if got := requestCount("udp", "egress", rawFlowEvent{Bytes: 128}); got != 1 {
		t.Fatalf("udp request count = %d, want 1", got)
	}
	if got := requestCount("tcp", "egress", rawFlowEvent{Bytes: 128}); got != 0 {
		t.Fatalf("tcp io request count = %d, want 0", got)
	}
}
