package service

import "testing"

func TestHiddenByGraphIPsDropsProtocolNoise(t *testing.T) {
	visibility := GraphVisibility{}
	for _, ip := range []string{"127.0.0.1", "::1", "ff02::2", "224.0.0.1", "fe80::1", "0.0.0.0", "::"} {
		if !hiddenByGraphIPs(visibility, "10.20.20.130", ip) {
			t.Fatalf("expected %s to be hidden", ip)
		}
	}
}

func TestHiddenByGraphIPsKeepsVMAddresses(t *testing.T) {
	visibility := GraphVisibility{}
	if hiddenByGraphIPs(visibility, "10.20.20.130", "10.20.20.199") {
		t.Fatal("expected private VM addresses to remain visible")
	}
}
