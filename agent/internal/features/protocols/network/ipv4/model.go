package ipv4

type Header struct {
	Source      string
	Destination string
	Protocol    uint8
	HeaderBytes int
}
