package ipv6

type Header struct {
	Source      string
	Destination string
	NextHeader  uint8
	HeaderBytes int
}
