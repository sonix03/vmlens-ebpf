package udp

import "encoding/binary"

func Parse(packet []byte) (Header, bool) {
	if len(packet) < 8 {
		return Header{}, false
	}
	return Header{
		SourcePort:      int(binary.BigEndian.Uint16(packet[0:2])),
		DestinationPort: int(binary.BigEndian.Uint16(packet[2:4])),
		Length:          int(binary.BigEndian.Uint16(packet[4:6])),
	}, true
}
