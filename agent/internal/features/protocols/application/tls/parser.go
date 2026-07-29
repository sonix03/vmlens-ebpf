package tls

func LooksLikeTLS(port int) bool {
	return port == 443
}
