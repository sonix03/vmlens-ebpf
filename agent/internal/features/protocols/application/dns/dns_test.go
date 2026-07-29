package dns

import "testing"

func TestLooksLikeDNS(t *testing.T) {
	if !LooksLikeDNS(53) {
		t.Fatal("expected port 53 to look like DNS")
	}
}
