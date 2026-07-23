package config

import "testing"

func TestDeepFlowDefaultNoiseFilters(t *testing.T) {
	t.Setenv("DEEPFLOW_EXCLUDED_PORTS", "")
	t.Setenv("DEEPFLOW_EXCLUDED_IPS", "")
	t.Setenv("DEEPFLOW_EXCLUDED_L7_RESOURCE_PREFIXES", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	wantPorts := map[int]bool{53: true, 123: true, 18080: true, 18081: true, 30033: true, 30035: true}
	for _, port := range cfg.DeepFlow.ExcludedPorts {
		delete(wantPorts, port)
	}
	if len(wantPorts) != 0 {
		t.Fatalf("missing default DeepFlow excluded ports: %#v", wantPorts)
	}
	wantIPs := map[string]bool{"10.20.20.125": true, "127.0.0.1": true}
	for _, ip := range cfg.DeepFlow.ExcludedIPs {
		delete(wantIPs, ip)
	}
	if len(wantIPs) != 0 {
		t.Fatalf("missing default DeepFlow excluded IPs: %#v", wantIPs)
	}
	wantPrefixes := map[string]bool{"/trident.": true, "trident.": true, "/api/agents/": true}
	for _, prefix := range cfg.DeepFlow.ExcludedL7ResourcePrefixes {
		delete(wantPrefixes, prefix)
	}
	if len(wantPrefixes) != 0 {
		t.Fatalf("missing default L7 resource prefixes: %#v", wantPrefixes)
	}
}

func TestDeepFlowRejectsInvalidExcludedPort(t *testing.T) {
	t.Setenv("DEEPFLOW_EXCLUDED_PORTS", "30033,70000")
	if _, err := Load(); err == nil {
		t.Fatal("expected validation error")
	}
}
