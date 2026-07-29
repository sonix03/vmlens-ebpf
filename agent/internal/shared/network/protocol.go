package network

type Protocol string

const (
	ProtocolUnknown Protocol = ""
	ProtocolTCP     Protocol = "tcp"
	ProtocolUDP     Protocol = "udp"
	ProtocolICMP    Protocol = "icmp"
)
