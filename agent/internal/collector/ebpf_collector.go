package collector

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
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
	SrcAddr     uint32
	DstAddr     uint32
	Connections uint32
	SrcPort     uint16
	DstPort     uint16
	Protocol    uint8
	Direction   uint8
}

type EBPFCollector struct {
	registration model.Registration
	collection   *cebpf.Collection
	links        []link.Link
	reader       *ringbuf.Reader
	closeOnce    sync.Once
}

func NewEBPF(registration model.Registration, objectPath string) (*EBPFCollector, error) {
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("remove memlock: %w", err)
	}
	spec, err := cebpf.LoadCollectionSpec(objectPath)
	if err != nil {
		return nil, fmt.Errorf("read eBPF object %q: %w", objectPath, err)
	}
	collection, err := cebpf.NewCollection(spec)
	if err != nil {
		return nil, fmt.Errorf("load eBPF object (root/CAP_BPF required): %w", err)
	}
	c := &EBPFCollector{registration: registration, collection: collection}
	fail := func(err error) (*EBPFCollector, error) { _ = c.Close(); return nil, err }

	attachments := []struct {
		program  string
		symbol   string
		ret      bool
		required bool
	}{
		{"trace_tcp_connect", "tcp_v4_connect", false, true},
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
	sourceIP, destinationIP := ipv4(raw.SrcAddr), ipv4(raw.DstAddr)
	if sourceIP == "0.0.0.0" && len(c.registration.PrivateIPs) > 0 {
		sourceIP = c.registration.PrivateIPs[0]
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
		Direction: direction, ConnectionCount: int64(raw.Connections), FirstSeen: now, LastSeen: now,
	}
	if direction == "ingress" {
		event.BytesReceived = int64(raw.Bytes)
	} else {
		event.BytesSent = int64(raw.Bytes)
	}
	if len(c.registration.Interfaces) > 0 {
		event.Interface = c.registration.Interfaces[0].Name
	}
	return event
}

func ipv4(value uint32) string {
	buffer := make([]byte, 4)
	binary.LittleEndian.PutUint32(buffer, value)
	return net.IP(buffer).String()
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
