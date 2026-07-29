package dns

func LooksLikeDNS(port int) bool {
	return port == 53
}
