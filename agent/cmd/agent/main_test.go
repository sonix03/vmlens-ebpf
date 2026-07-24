package main

import (
	"testing"
	"time"

	"github.com/vmlens/vmlens/agent/internal/telemetry"
)

func TestEndpointFilterIncludesConfiguredTunnelPeer(t *testing.T) {
	filter := newEndpointFilter("http://127.0.0.1:18080", []string{"10.20.20.125"})
	for _, ip := range []string{"127.0.0.1", "10.20.20.125"} {
		if !filter.matches(ip) {
			t.Fatalf("expected %s to be ignored", ip)
		}
	}
	if filter.matches("10.20.20.130") {
		t.Fatal("peer VM must not be ignored")
	}
}

func TestFlowFilterAllowAndDenyCIDRs(t *testing.T) {
	filter, err := newFlowFilter([]string{"10.20.20.0/24", "203.0.113.10"}, []string{"10.20.20.125/32"})
	if err != nil {
		t.Fatal(err)
	}
	if !filter.allows(telemetry.FlowEvent{SrcIP: "10.20.20.130", DstIP: "198.51.100.20"}) {
		t.Fatal("source in allow CIDR should pass")
	}
	if !filter.allows(telemetry.FlowEvent{SrcIP: "10.30.30.10", DstIP: "203.0.113.10"}) {
		t.Fatal("single IP allow CIDR should pass")
	}
	if filter.allows(telemetry.FlowEvent{SrcIP: "10.20.20.130", DstIP: "10.20.20.125"}) {
		t.Fatal("deny CIDR should override allow CIDR")
	}
	if filter.allows(telemetry.FlowEvent{SrcIP: "10.30.30.10", DstIP: "198.51.100.20"}) {
		t.Fatal("flow outside allow CIDRs should be dropped")
	}
}

func TestIgnoreFlowDropsProbePortWithoutDroppingPeerVM(t *testing.T) {
	controlPlane := newEndpointFilter("http://127.0.0.1:18080", []string{"10.20.20.125"})
	peerTraffic := telemetry.FlowEvent{SrcIP: "10.20.20.130", DstIP: "10.20.20.199", SrcPort: 43000, DstPort: 8081}
	if ignoreFlow(controlPlane, peerTraffic, []int{18081}) {
		t.Fatal("normal peer traffic must stay visible")
	}
	probeTraffic := telemetry.FlowEvent{SrcIP: "10.20.20.130", DstIP: "10.20.20.199", SrcPort: 43000, DstPort: 18081}
	if !ignoreFlow(controlPlane, probeTraffic, []int{18081}) {
		t.Fatal("vmlens probe port must be ignored by flow metrics")
	}
}

func TestIgnoreFlowDropsControlPlane(t *testing.T) {
	controlPlane := newEndpointFilter("http://127.0.0.1:18080", []string{"10.20.20.125"})
	event := telemetry.FlowEvent{SrcIP: "10.20.20.130", DstIP: "10.20.20.125", SrcPort: 43000, DstPort: 8080}
	if !ignoreFlow(controlPlane, event, nil) {
		t.Fatal("control-plane destination must be ignored")
	}
}

func TestFlowAccumulatorPreservesByteTotals(t *testing.T) {
	accumulator := newFlowAccumulator()
	first := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	last := first.Add(time.Second)

	accumulator.Add(telemetry.FlowEvent{
		AgentID: "agent-a", SrcIP: "10.20.20.130", DstIP: "140.82.121.4",
		SrcPort: 45000, DstPort: 443, Protocol: "tcp", Direction: "ingress",
		BytesReceived: 8 * 1024 * 1024, ConnectionCount: 2, RequestCount: 2, FirstSeen: first, LastSeen: first,
	})
	accumulator.Add(telemetry.FlowEvent{
		AgentID: "agent-a", SrcIP: "10.20.20.130", DstIP: "140.82.121.4",
		SrcPort: 45000, DstPort: 443, Protocol: "tcp", Direction: "ingress",
		BytesReceived: 17 * 1024 * 1024, ConnectionCount: 3, RequestCount: 3, FirstSeen: last, LastSeen: last,
	})

	batch := accumulator.Drain()
	if len(batch) != 1 {
		t.Fatalf("expected one aggregated flow, got %d", len(batch))
	}
	if got, want := batch[0].BytesReceived, int64(25*1024*1024); got != want {
		t.Fatalf("received bytes = %d, want %d", got, want)
	}
	if got, want := batch[0].RequestCount, int64(5); got != want {
		t.Fatalf("request count = %d, want %d", got, want)
	}
	if !batch[0].FirstSeen.Equal(first) || !batch[0].LastSeen.Equal(last) {
		t.Fatalf("unexpected time window: %s - %s", batch[0].FirstSeen, batch[0].LastSeen)
	}
	if len(accumulator.Drain()) != 0 {
		t.Fatal("drain must reset the accumulator")
	}
}

func TestFlowAccumulatorKeepsDirectionsSeparate(t *testing.T) {
	accumulator := newFlowAccumulator()
	base := telemetry.FlowEvent{
		AgentID: "agent-a", SrcIP: "10.20.20.130", DstIP: "140.82.121.4",
		DstPort: 443, Protocol: "tcp",
	}
	ingress := base
	ingress.Direction = "ingress"
	ingress.BytesReceived = 1024
	egress := base
	egress.Direction = "egress"
	egress.BytesSent = 512
	accumulator.Add(ingress)
	accumulator.Add(egress)

	if got := len(accumulator.Drain()); got != 2 {
		t.Fatalf("expected separate ingress and egress flows, got %d", got)
	}
}
