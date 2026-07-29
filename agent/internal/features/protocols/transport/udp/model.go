package udp

type Header struct {
	SourcePort      int
	DestinationPort int
	Length          int
}
