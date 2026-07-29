package classification

const (
	ProtocolTCP  = "tcp"
	ProtocolUDP  = "udp"
	ProtocolICMP = "icmp"
)

type Model struct {
	Network     string
	Transport   string
	Application string
}
