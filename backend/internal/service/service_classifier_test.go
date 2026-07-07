package service

import "testing"

func TestClassifyKnownDevelopmentServices(t *testing.T) {
	tests := []struct {
		protocol  string
		direction string
		srcPort   int
		dstPort   int
		wantName  string
		wantPort  int
	}{
		{"tcp", "egress", 45000, 6379, "redis", 6379},
		{"tcp", "ingress", 6379, 45000, "redis", 6379},
		{"tcp", "egress", 45001, 5672, "rabbitmq", 5672},
		{"tcp", "egress", 45002, 5432, "postgresql", 5432},
		{"tcp", "egress", 45003, 6443, "kubernetes-api", 6443},
		{"udp", "ingress", 53000, 53, "dns", 53},
		{"tcp", "egress", 45004, 31080, "kubernetes-nodeport", 31080},
	}
	for _, test := range tests {
		name, port := classifyService(test.protocol, test.direction, test.srcPort, test.dstPort)
		if name != test.wantName || port != test.wantPort {
			t.Fatalf("classify %s %s %d->%d = %s/%d, want %s/%d", test.protocol, test.direction, test.srcPort, test.dstPort, name, port, test.wantName, test.wantPort)
		}
	}
}

func TestClassifyUnknownServiceKeepsObservedPort(t *testing.T) {
	name, port := classifyService("tcp", "egress", 45000, 18080)
	if name != "tcp/18080" || port != 18080 {
		t.Fatalf("unexpected custom service %s/%d", name, port)
	}
}
