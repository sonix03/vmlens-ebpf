package tls

import "testing"

func TestLooksLikeTLS(t *testing.T) {
	if !LooksLikeTLS(443) {
		t.Fatal("expected port 443 to look like TLS")
	}
}
