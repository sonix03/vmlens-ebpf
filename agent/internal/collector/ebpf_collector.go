package collector

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"

	"github.com/vmlens/vmlens/agent/internal/model"
)

type rawFlowEvent struct {
	TimestampNS uint64
	Bytes       uint64
	SrcAddr     [16]byte
	DstAddr     [16]byte
	Connections uint32
	SrcPort     uint16
	DstPort     uint16
	Family      uint16
	Protocol    uint8
	Direction   uint8
	Packets     uint32
}

type EBPFOptions struct {
	ObjectPath       string
	CaptureMode      string
	CaptureInterface string
}

type EBPFCollector struct {
	registration model.Registration
	captureMode  string
	ifaceName    string
	collection   *cebpf.Collection
	links        []link.Link
	reader       *ringbuf.Reader
	closeOnce    sync.Once
}

func NewEBPF(registration model.Registration, options EBPFOptions) (*EBPFCollector, error) {
	mode := strings.ToLower(strings.TrimSpace(options.CaptureMode))
	if mode == "" {
		mode = "auto"
	}
	if mode != "auto" && mode != "tc" && mode != "kprobe" {
		return nil, fmt.Errorf("unsupported capture mode %q", options.CaptureMode)
	}
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("remove memlock: %w", err)
	}
	spec, err := cebpf.LoadCollectionSpec(options.ObjectPath)
	if err != nil {
		return nil, fmt.Errorf("read eBPF object %q: %w", options.ObjectPath, err)
	}
	collection, err := cebpf.NewCollection(spec)
	if err != nil {
		return nil, fmt.Errorf("load eBPF object (root/CAP_BPF required): %w", err)
	}
	c := &EBPFCollector{
		registration: registration,
		captureMode:  mode,
		ifaceName:    strings.TrimSpace(options.CaptureInterface),
		collection:   collection,
	}
	fail := func(err error) (*EBPFCollector, error) { _ = c.Close(); return nil, err }

	if mode == "tc" || mode == "auto" {
		if err := c.attachTCX(); err == nil {
			if eventsMap := collection.Maps["events"]; eventsMap != nil {
				c.reader, err = ringbuf.NewReader(eventsMap)
				if err != nil {
					return fail(err)
				}
				return c, nil
			}
			return fail(fmt.Errorf("events ring buffer missing"))
		} else if mode == "tc" {
			return fail(err)
		}
	}

	if mode == "tc" {
		return fail(fmt.Errorf("tc capture requested but no TCX program was attached"))
	}

	attachments := []struct {
		program  string
		symbol   string
		ret      bool
		required bool
	}{
		{"trace_tcp_connect", "tcp_v4_connect", false, true},
		{"trace_tcp_v6_connect", "tcp_v6_connect", false, false},
		{"trace_tcp_accept", "inet_csk_accept", true, true},
		{"trace_tcp_send", "tcp_sendmsg", false, true},
		{"trace_tcp_send_ret", "tcp_sendmsg", true, true},
		{"trace_tcp_recv", "tcp_recvmsg", false, true},
		{"trace_tcp_recv_ret", "tcp_recvmsg", true, true},
		{"trace_udp_send", "udp_sendmsg", false, false},
		{"trace_udp_send_ret", "udp_sendmsg", true, false},
		{"trace_udp_recv", "udp_recvmsg", false, false},
		{"trace_udp_recv_ret", "udp_recvmsg", true, false},
	}
	for _, item := range attachments {
		program := collection.Programs[item.program]
		if program == nil {
			if item.required {
				return fail(fmt.Errorf("program %s missing", item.program))
			}
			continue
		}
		var attached link.Link
		if item.ret {
			attached, err = link.Kretprobe(item.symbol, program, nil)
		} else {
			attached, err = link.Kprobe(item.symbol, program, nil)
		}
		if err != nil {
			if item.required {
				return fail(fmt.Errorf("attach %s: %w", item.program, err))
			}
			continue
		}
		c.links = append(c.links, attached)
	}
	eventsMap := collection.Maps["events"]
	if eventsMap == nil {
		return fail(fmt.Errorf("events ring buffer missing"))
	}
	c.reader, err = ringbuf.NewReader(eventsMap)
	if err != nil {
		return fail(err)
	}
	return c, nil
}

func (c *EBPFCollector) attachTCX() error {
	ifaceName := c.ifaceName
	if ifaceName == "" {
		ifaceName = c.defaultInterfaceName()
	}
	if ifaceName == "" {
		return fmt.Errorf("capture interface is empty")
	}
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return fmt.Errorf("lookup capture interface %q: %w", ifaceName, err)
	}
	ingress := c.collection.Programs["tc_ingress"]
	egress := c.collection.Programs["tc_egress"]
	if ingress == nil || egress == nil {
		return fmt.Errorf("tc_ingress/tc_egress programs missing")
	}
	ingressLink, err := link.AttachTCX(link.TCXOptions{
		Interface: iface.Index,
		Program:   ingress,
		Attach:    cebpf.AttachTCXIngress,
	})
	if err != nil {
		return fmt.Errorf("attach tc ingress on %s: %w", ifaceName, err)
	}
	egressLink, err := link.AttachTCX(link.TCXOptions{
		Interface: iface.Index,
		Program:   egress,
		Attach:    cebpf.AttachTCXEgress,
	})
	if err != nil {
		_ = ingressLink.Close()
		return fmt.Errorf("attach tc egress on %s: %w", ifaceName, err)
	}
	c.links = append(c.links, ingressLink)
	c.links = append(c.links, egressLink)
	c.captureMode = "tc"
	c.ifaceName = ifaceName
	return nil
}

func (c *EBPFCollector) Run(ctx context.Context) (<-chan model.FlowEvent, <-chan error) {
	events := make(chan model.FlowEvent, 1024)
	errorsChannel := make(chan error, 8)
	go func() {
		defer close(events)
		defer close(errorsChannel)
		go func() { <-ctx.Done(); _ = c.Close() }()
		for {
			record, err := c.reader.Read()
			if err != nil {
				if !errors.Is(err, ringbuf.ErrClosed) {
					errorsChannel <- err
				}
				return
			}
			var raw rawFlowEvent
			if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &raw); err != nil {
				errorsChannel <- err
				continue
			}
			events <- c.convert(raw)
		}
	}()
	return events, errorsChannel
}

func (c *EBPFCollector) convert(raw rawFlowEvent) model.FlowEvent {
	sourceIP, destinationIP := socketIP(raw.SrcAddr, raw.Family), socketIP(raw.DstAddr, raw.Family)
	if parsed := net.ParseIP(sourceIP); parsed == nil || parsed.IsUnspecified() {
		if fallback := c.fallbackSource(raw.Family); fallback != "" {
			sourceIP = fallback
		}
	}
	protocol := "tcp"
	if raw.Protocol == 17 {
		protocol = "udp"
	}
	direction := "egress"
	if raw.Direction == 1 {
		direction = "ingress"
	}
	now := time.Now().UTC()
	event := model.FlowEvent{
		AgentID: c.registration.AgentID, SrcIP: sourceIP, DstIP: destinationIP,
		SrcPort: int(raw.SrcPort), DstPort: int(raw.DstPort), Protocol: protocol,
		Direction: direction, ConnectionCount: int64(raw.Connections), RequestCount: requestCount(protocol, direction, raw),
		Packets: int64(raw.Packets), FirstSeen: now, LastSeen: now,
	}
	if direction == "ingress" {
		event.BytesReceived = int64(raw.Bytes)
	} else {
		event.BytesSent = int64(raw.Bytes)
	}
	if c.ifaceName != "" {
		event.Interface = c.ifaceName
	} else if len(c.registration.Interfaces) > 0 {
		event.Interface = c.registration.Interfaces[0].Name
	}
	return event
}

func (c *EBPFCollector) defaultInterfaceName() string {
	for _, iface := range c.registration.Interfaces {
		if iface.Name != "" {
			return iface.Name
		}
	}
	return ""
}

func requestCount(protocol, direction string, raw rawFlowEvent) int64 {
	if raw.Connections > 0 {
		return int64(raw.Connections)
	}
	if protocol != "udp" || raw.Bytes == 0 {
		return 0
	}
	switch direction {
	case "egress", "ingress":
		return 1
	default:
		return 0
	}
}

func socketIP(value [16]byte, family uint16) string {
	if family == 10 {
		return net.IP(value[:]).String()
	}
	return net.IP(value[:4]).String()
}

func (c *EBPFCollector) fallbackSource(family uint16) string {
	if family == 10 {
		for _, iface := range c.registration.Interfaces {
			ip := net.ParseIP(iface.IPAddress)
			if ip != nil && ip.To4() == nil && !ip.IsUnspecified() {
				return ip.String()
			}
		}
		return ""
	}
	if len(c.registration.PrivateIPs) > 0 {
		return c.registration.PrivateIPs[0]
	}
	return ""
}

func (c *EBPFCollector) Close() error {
	var first error
	c.closeOnce.Do(func() {
		if c.reader != nil {
			first = c.reader.Close()
		}
		for _, attached := range c.links {
			if err := attached.Close(); err != nil && first == nil {
				first = err
			}
		}
		if c.collection != nil {
			c.collection.Close()
		}
	})
	return first
}
