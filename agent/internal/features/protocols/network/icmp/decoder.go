package icmp

import "fmt"

func DecodePacket(packet []byte) (Event, error) {
	if source, destination := parseIPv4ICMP(packet); source != "" && destination != "" {
		return Event{SourceIP: source, DestinationIP: destination, Bytes: int64(len(packet))}, nil
	}
	if source, destination := parseIPv6ICMP(packet); source != "" && destination != "" {
		return Event{SourceIP: source, DestinationIP: destination, Bytes: int64(len(packet))}, nil
	}
	return Event{}, fmt.Errorf("packet is not ICMP")
}
