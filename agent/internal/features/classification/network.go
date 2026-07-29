package classification

const (
	NetworkIPv4 = "ipv4"
	NetworkIPv6 = "ipv6"
)

func NetworkFromFamily(family uint16) string {
	switch family {
	case 2:
		return NetworkIPv4
	case 10:
		return NetworkIPv6
	default:
		return ""
	}
}
