package deepflow

import (
	"testing"
	"time"

	"github.com/vmlens/vmlens/backend/internal/model"
)

func TestNormalizeTopologyKeepsInventoryNodesWithoutTraffic(t *testing.T) {
	topology := NormalizeTopology([]model.VM{{ID: "vm-1", Name: "testing-a-2", PrivateIP: "10.20.20.130", Status: "online"}}, nil, nil, nil, TopologyOptions{Window: time.Minute})
	if len(topology.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(topology.Nodes))
	}
	if topology.Nodes[0].ID != "vm-1" {
		t.Fatalf("node id = %s, want vm-1", topology.Nodes[0].ID)
	}
	if len(topology.Edges) != 0 {
		t.Fatalf("edges = %d, want 0", len(topology.Edges))
	}
}

func TestNormalizeTopologyDeduplicatesL7ByObservationPriority(t *testing.T) {
	baseTime := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	vms := []model.VM{
		{ID: "vm-a", Name: "client", PrivateIP: "10.20.20.130", Status: "online"},
		{ID: "vm-b", Name: "server", PrivateIP: "10.20.20.199", Status: "online"},
	}
	l7Rows := []model.DeepFlowL7Request{
		{Time: baseTime, SourceIP: "10.20.20.130", DestIP: "10.20.20.199", RequestType: "GET", RequestResource: "/", ResponseCode: 200, ResponseLength: 10, AgentID: "1", ObservationPoint: "c", InternetDirection: "0 -> 0"},
		{Time: baseTime.Add(5 * time.Millisecond), SourceIP: "10.20.20.130", DestIP: "10.20.20.199", RequestType: "GET", RequestResource: "/", ResponseCode: 200, ResponseLength: 10, AgentID: "2", ObservationPoint: "s-p", InternetDirection: "0 -> 0"},
	}

	topology := NormalizeTopology(vms, nil, l7Rows, nil, TopologyOptions{Window: time.Minute})
	if len(topology.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(topology.Edges))
	}
	edge := topology.Edges[0]
	if edge.RequestCount != 1 {
		t.Fatalf("request_count = %d, want 1", edge.RequestCount)
	}
	if len(edge.ObservationPoints) != 1 || edge.ObservationPoints[0] != "s-p" {
		t.Fatalf("observation_points = %#v, want [s-p]", edge.ObservationPoints)
	}
}

func TestNormalizeTopologyExternalTrafficCreatesExternalNode(t *testing.T) {
	now := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	vms := []model.VM{{ID: "vm-a", Name: "client", PrivateIP: "10.20.20.130", Status: "online"}}
	l4Rows := []model.DeepFlowL4Flow{{
		Time: now, SourceIP: "10.20.20.130", DestIP: "1.1.1.1", ClientPort: 45000, ServerPort: 443,
		Protocol: "tcp", Status: "Success", ByteTX: 100, ByteRX: 200, TotalBytes: 300,
		AgentID: "1", L3EPCID0: 1, L3EPCID1: -2, InternetDirection: "0 -> 1",
	}}

	topology := NormalizeTopology(vms, l4Rows, nil, nil, TopologyOptions{Window: time.Minute, MaskExternalIPs: true})
	if len(topology.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(topology.Edges))
	}
	edge := topology.Edges[0]
	if edge.Direction != directionInternalExternal {
		t.Fatalf("direction = %s, want %s", edge.Direction, directionInternalExternal)
	}
	if edge.DestRole != roleExternalIP {
		t.Fatalf("dest_role = %s, want %s", edge.DestRole, roleExternalIP)
	}
	if edge.DestIP == "1.1.1.1" {
		t.Fatal("external IP should be masked")
	}
	if edge.Kind != edgeKindTraffic {
		t.Fatalf("kind = %s, want %s", edge.Kind, edgeKindTraffic)
	}
}

func TestNormalizeTopologyICMPCreatesReachabilityEdge(t *testing.T) {
	now := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	vms := []model.VM{
		{ID: "vm-a", Name: "testing-a-1", PrivateIP: "10.20.20.199", Status: "online"},
		{ID: "vm-b", Name: "testing-a-extra", PrivateIP: "10.20.20.249", Status: "online"},
	}
	l4Rows := []model.DeepFlowL4Flow{{
		Time: now, SourceIP: "10.20.20.199", DestIP: "10.20.20.249", ServerPort: 0,
		Protocol: "icmp", Status: "Success", ByteTX: 84, ByteRX: 84, TotalBytes: 168,
		RTTMs: 0.586, AgentID: "2", L3EPCID0: 1, L3EPCID1: 1, InternetDirection: "0 -> 0",
	}}

	topology := NormalizeTopology(vms, l4Rows, nil, nil, TopologyOptions{Window: time.Minute})
	if len(topology.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(topology.Edges))
	}
	edge := topology.Edges[0]
	if edge.Kind != edgeKindReachability {
		t.Fatalf("kind = %s, want %s", edge.Kind, edgeKindReachability)
	}
	if !edge.Reachable {
		t.Fatal("reachable = false, want true")
	}
	if edge.Protocol != "icmp" {
		t.Fatalf("protocol = %s, want icmp", edge.Protocol)
	}
	if edge.RequestCount != 1 {
		t.Fatalf("request_count = %d, want 1", edge.RequestCount)
	}
	if edge.AvgRTTMs != 0.586 {
		t.Fatalf("avg_rtt_ms = %f, want 0.586", edge.AvgRTTMs)
	}
}
