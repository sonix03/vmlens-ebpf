package tls

func FromHandshake(handshake Handshake) Metadata {
	return Metadata{ServerName: handshake.ServerName, Version: handshake.Version}
}
