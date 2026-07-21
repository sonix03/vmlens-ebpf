//go:build linux

package capture

import (
	"encoding/binary"
	"net"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/vmlens/vmlens/agent/internal/telemetry"
)

func TestICMPCollectorParsesEgressPing(t *testing.T) {
	collector := ICMPCollector{registration: telemetry.Registration{AgentID: "agent-a"}, ifaceName: "ens3"}
	event, ok := collector.parse(ipv4ICMPPacket("10.20.20.130", "10.20.20.249"), &unix.SockaddrLinklayer{Pkttype: packetOutgoing})
	if !ok {
		t.Fatal("expected ICMP packet to parse")
	}
	if event.Protocol != "icmp" || event.Direction != "egress" {
		t.Fatalf("unexpected protocol/direction: %s/%s", event.Protocol, event.Direction)
	}
	if event.SrcIP != "10.20.20.130" || event.DstIP != "10.20.20.249" {
		t.Fatalf("unexpected route: %s -> %s", event.SrcIP, event.DstIP)
	}
	if event.BytesSent == 0 || event.BytesReceived != 0 || event.RequestCount != 1 || event.DstPort != 0 {
		t.Fatalf("unexpected counters: %+v", event)
	}
}

func TestICMPCollectorParsesIngressFromLocalPerspective(t *testing.T) {
	collector := ICMPCollector{registration: telemetry.Registration{AgentID: "agent-b"}, ifaceName: "ens3"}
	event, ok := collector.parse(ipv4ICMPPacket("10.20.20.130", "10.20.20.249"), &unix.SockaddrLinklayer{Pkttype: unix.PACKET_HOST})
	if !ok {
		t.Fatal("expected ICMP packet to parse")
	}
	if event.Protocol != "icmp" || event.Direction != "ingress" {
		t.Fatalf("unexpected protocol/direction: %s/%s", event.Protocol, event.Direction)
	}
	if event.SrcIP != "10.20.20.249" || event.DstIP != "10.20.20.130" {
		t.Fatalf("unexpected local-perspective route: %s -> %s", event.SrcIP, event.DstIP)
	}
	if event.BytesReceived == 0 || event.BytesSent != 0 || event.RequestCount != 1 {
		t.Fatalf("unexpected counters: %+v", event)
	}
}

func ipv4ICMPPacket(source, destination string) []byte {
	packet := make([]byte, ethernetHeaderLen+20+8)
	binary.BigEndian.PutUint16(packet[12:14], ethPIP)
	ip := packet[ethernetHeaderLen:]
	ip[0] = 0x45
	ip[9] = ipProtocolICMP
	copy(ip[12:16], net.ParseIP(source).To4())
	copy(ip[16:20], net.ParseIP(destination).To4())
	return packet
}
