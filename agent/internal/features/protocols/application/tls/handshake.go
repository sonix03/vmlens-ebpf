package tls

type Handshake struct {
	ServerName string
	Version    string
	Success    bool
}
