package main

import "testing"

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
