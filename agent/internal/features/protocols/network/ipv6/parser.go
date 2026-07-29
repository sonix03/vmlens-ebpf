package ipv6

import "net"

func Parse(packet []byte) (Header, bool) {
	if len(packet) < 40 || packet[0]>>4 != 6 {
		return Header{}, false
	}
	return Header{
		Source:      net.IP(packet[8:24]).String(),
		Destination: net.IP(packet[24:40]).String(),
		NextHeader:  packet[6],
		HeaderBytes: 40,
	}, true
}
