package ipv4

import "net"

func Parse(packet []byte) (Header, bool) {
	if len(packet) < 20 || packet[0]>>4 != 4 {
		return Header{}, false
	}
	headerLen := int(packet[0]&0x0f) * 4
	if headerLen < 20 || len(packet) < headerLen {
		return Header{}, false
	}
	return Header{
		Source:      net.IP(packet[12:16]).String(),
		Destination: net.IP(packet[16:20]).String(),
		Protocol:    packet[9],
		HeaderBytes: headerLen,
	}, true
}
