package packet

import (
	"net"
	"testing"

	telemetry "github.com/vmlens/vmlens/agent/internal/exporter"
	"github.com/vmlens/vmlens/agent/internal/features/classification"
	tcpconnection "github.com/vmlens/vmlens/agent/internal/features/protocols/transport/tcp/connection"
	"github.com/vmlens/vmlens/agent/internal/features/traffic/direction"
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
	if got := tcpconnection.InferRequestCount(classification.ProtocolTCP, direction.Egress, 0, 3, 0); got != 3 {
		t.Fatalf("tcp request count = %d, want 3", got)
	}
	if got := tcpconnection.InferRequestCount(classification.ProtocolTCP, direction.Egress, 0, 0, 1); got != 0 {
		t.Fatalf("tcp rst request count = %d, want 0", got)
	}
	if got := tcpconnection.InferRequestCount(classification.ProtocolUDP, direction.Egress, 128, 0, 0); got != 1 {
		t.Fatalf("udp request count = %d, want 1", got)
	}
	if got := tcpconnection.InferRequestCount(classification.ProtocolICMP, direction.Egress, 84, 0, 0); got != 1 {
		t.Fatalf("icmp request count = %d, want 1", got)
	}
	if got := tcpconnection.InferRequestCount(classification.ProtocolTCP, direction.Egress, 128, 0, 0); got != 0 {
		t.Fatalf("tcp io request count = %d, want 0", got)
	}
}

func TestIgnoredPortKeysDropsInvalidAndDuplicatePorts(t *testing.T) {
	got := ignoredPortKeys([]int{0, -1, 22, 22, 18081, 70000, 65535})
	want := []uint32{22, 18081, 65535}
	if len(got) != len(want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("keys = %v, want %v", got, want)
		}
	}
}
