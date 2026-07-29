package classification

const (
	ApplicationHTTP       = "http"
	ApplicationHTTPS      = "https"
	ApplicationDNS        = "dns"
	ApplicationSSH        = "ssh"
	ApplicationPostgreSQL = "postgresql"
)

func ApplicationFromPort(protocol string, dstPort int) string {
	switch {
	case protocol == ProtocolTCP && dstPort == 80:
		return ApplicationHTTP
	case protocol == ProtocolTCP && dstPort == 443:
		return ApplicationHTTPS
	case protocol == ProtocolTCP && dstPort == 22:
		return ApplicationSSH
	case protocol == ProtocolTCP && dstPort == 5432:
		return ApplicationPostgreSQL
	case dstPort == 53:
		return ApplicationDNS
	default:
		return ""
	}
}
