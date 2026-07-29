package http

import "testing"

func TestLooksLikeRequest(t *testing.T) {
	if !LooksLikeRequest([]byte("GET / HTTP/1.1\r\n")) {
		t.Fatal("expected GET payload to look like HTTP request")
	}
}
