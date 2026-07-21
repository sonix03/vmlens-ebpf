//go:build linux

package capture

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/vmlens/vmlens/agent/internal/telemetry"
)

const (
	ethernetHeaderLen = 14
	ethPIP            = 0x0800
	ethPIPv6          = 0x86dd
	ipProtocolICMP    = 1
	ipProtocolICMPv6  = 58
	packetOutgoing    = 4
)

type ICMPCollector struct {
	registration telemetry.Registration
	ifaceName    string
	fd           int
	closeOnce    sync.Once
}

func NewICMP(registration telemetry.Registration, ifaceName string) (*ICMPCollector, error) {
	ifaceName = firstNonEmpty(ifaceName, firstInterfaceName(registration))
	if ifaceName == "" {
		return nil, fmt.Errorf("icmp capture interface is empty")
	}
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("lookup icmp capture interface %q: %w", ifaceName, err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		return nil, fmt.Errorf("open packet socket for icmp capture: %w", err)
	}
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  iface.Index,
	}); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("bind packet socket on %s: %w", ifaceName, err)
	}
	return &ICMPCollector{registration: registration, ifaceName: ifaceName, fd: fd}, nil
}

func (c *ICMPCollector) Run(ctx context.Context) (<-chan telemetry.FlowEvent, <-chan error) {
	events := make(chan telemetry.FlowEvent, 256)
	errorsChannel := make(chan error, 4)
	go func() {
		defer close(events)
		defer close(errorsChannel)
		go func() { <-ctx.Done(); _ = c.Close() }()
		buffer := make([]byte, 65535)
		for {
			n, sockaddr, err := unix.Recvfrom(c.fd, buffer, 0)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if err == unix.EBADF || err == unix.EINVAL {
					return
				}
				errorsChannel <- fmt.Errorf("read icmp packet: %w", err)
				continue
			}
			event, ok := c.parse(buffer[:n], sockaddr)
			if !ok {
				continue
			}
			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return events, errorsChannel
}

func (c *ICMPCollector) Close() error {
	var err error
	c.closeOnce.Do(func() {
		err = unix.Close(c.fd)
	})
	return err
}

func (c *ICMPCollector) parse(packet []byte, sockaddr unix.Sockaddr) (telemetry.FlowEvent, bool) {
	if len(packet) < ethernetHeaderLen {
		return telemetry.FlowEvent{}, false
	}
	direction := "ingress"
	if linkLayer, ok := sockaddr.(*unix.SockaddrLinklayer); ok && linkLayer.Pkttype == packetOutgoing {
		direction = "egress"
	}

	ethProtocol := binary.BigEndian.Uint16(packet[12:14])
	var sourceIP, destinationIP string
	switch ethProtocol {
	case ethPIP:
		sourceIP, destinationIP = parseIPv4ICMP(packet[ethernetHeaderLen:])
	case ethPIPv6:
		sourceIP, destinationIP = parseIPv6ICMP(packet[ethernetHeaderLen:])
	default:
		return telemetry.FlowEvent{}, false
	}
	if sourceIP == "" || destinationIP == "" {
		return telemetry.FlowEvent{}, false
	}

	if direction == "ingress" {
		sourceIP, destinationIP = destinationIP, sourceIP
	}
	now := time.Now().UTC()
	event := telemetry.FlowEvent{
		AgentID:      c.registration.AgentID,
		SrcIP:        sourceIP,
		DstIP:        destinationIP,
		Protocol:     "icmp",
		Direction:    direction,
		Packets:      1,
		RequestCount: 1,
		FirstSeen:    now,
		LastSeen:     now,
		Interface:    c.ifaceName,
	}
	if direction == "ingress" {
		event.BytesReceived = int64(len(packet))
	} else {
		event.BytesSent = int64(len(packet))
	}
	return event, true
}

func parseIPv4ICMP(packet []byte) (string, string) {
	if len(packet) < 20 {
		return "", ""
	}
	versionIHL := packet[0]
	if versionIHL>>4 != 4 {
		return "", ""
	}
	headerLen := int(versionIHL&0x0f) * 4
	if headerLen < 20 || len(packet) < headerLen+1 {
		return "", ""
	}
	if packet[9] != ipProtocolICMP {
		return "", ""
	}
	return net.IP(packet[12:16]).String(), net.IP(packet[16:20]).String()
}

func parseIPv6ICMP(packet []byte) (string, string) {
	if len(packet) < 40 {
		return "", ""
	}
	if packet[0]>>4 != 6 || packet[6] != ipProtocolICMPv6 {
		return "", ""
	}
	return net.IP(packet[8:24]).String(), net.IP(packet[24:40]).String()
}

func firstInterfaceName(registration telemetry.Registration) string {
	for _, iface := range registration.Interfaces {
		if iface.Name != "" {
			return iface.Name
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func htons(value uint16) uint16 {
	return (value << 8) | (value >> 8)
}
