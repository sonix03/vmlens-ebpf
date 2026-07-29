package http

import "bytes"

func LooksLikeRequest(payload []byte) bool {
	return bytes.HasPrefix(payload, []byte("GET ")) ||
		bytes.HasPrefix(payload, []byte("POST ")) ||
		bytes.HasPrefix(payload, []byte("PUT ")) ||
		bytes.HasPrefix(payload, []byte("DELETE ")) ||
		bytes.HasPrefix(payload, []byte("PATCH "))
}
