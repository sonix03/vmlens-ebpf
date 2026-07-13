package ebpf

import (
	"bytes"
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
)

const (
	EventExec    = 1
	EventConnect = 2
	EventAccept  = 3
	EventSend    = 4
	EventReceive = 5
)

type Event struct {
	Timestamp       time.Time
	Type            uint32
	PID             uint32
	PPID            uint32
	UID             uint32
	GID             uint32
	Direction       string
	Protocol        string
	SrcIP           string
	SrcPort         uint16
	DstIP           string
	DstPort         uint16
	Bytes           uint64
	Packets         uint32
	ConnectionCount uint32
	Comm            string
	Filename        string
}

// rawEvent mirrors types.h. binary.Read processes fields in declaration order;
// the kernel record may contain tail padding, which is intentionally ignored.
type rawEvent struct {
	TimestampNS uint64
	Bytes       uint64
	Type        uint32
	PID         uint32
	PPID        uint32
	UID         uint32
	GID         uint32
	Packets     uint32
	Connections uint32
	Family      uint16
	SrcPort     uint16
	DstPort     uint16
	Direction   uint8
	Protocol    uint8
	SrcAddr     [16]byte
	DstAddr     [16]byte
	Comm        [16]byte
	Filename    [256]byte
}

type Handle struct {
	collection *cebpf.Collection
	links      []link.Link
	reader     *ringbuf.Reader
	closeOnce  sync.Once
	Events     chan Event
	Errors     chan error
}

func Load(objectPath string) (*Handle, error) {
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("remove memlock limit: %w", err)
	}
	spec, err := cebpf.LoadCollectionSpec(objectPath)
	if err != nil {
		return nil, fmt.Errorf("read eBPF object %q: %w", objectPath, err)
	}
	coll, err := cebpf.NewCollection(spec)
	if err != nil {
		return nil, fmt.Errorf("load eBPF collection (root/CAP_BPF and kernel BTF may be required): %w", err)
	}
	h := &Handle{
		collection: coll,
		Events:     make(chan Event, 4096),
		Errors:     make(chan error, 8),
	}
	fail := func(err error) (*Handle, error) {
		_ = h.Close()
		return nil, err
	}
	attachments := []struct {
		program string
		attach  func(*cebpf.Program) (link.Link, error)
	}{
		{"trace_exec", func(p *cebpf.Program) (link.Link, error) {
			return link.Tracepoint("syscalls", "sys_enter_execve", p, nil)
		}},
		{"trace_tcp_connect", func(p *cebpf.Program) (link.Link, error) { return link.Kprobe("tcp_v4_connect", p, nil) }},
		{"trace_tcp_accept", func(p *cebpf.Program) (link.Link, error) { return link.Kretprobe("inet_csk_accept", p, nil) }},
		{"trace_tcp_sendmsg", func(p *cebpf.Program) (link.Link, error) { return link.Kprobe("tcp_sendmsg", p, nil) }},
		{"trace_tcp_sendmsg_return", func(p *cebpf.Program) (link.Link, error) { return link.Kretprobe("tcp_sendmsg", p, nil) }},
		{"trace_tcp_recvmsg", func(p *cebpf.Program) (link.Link, error) { return link.Kprobe("tcp_recvmsg", p, nil) }},
		{"trace_tcp_recvmsg_return", func(p *cebpf.Program) (link.Link, error) { return link.Kretprobe("tcp_recvmsg", p, nil) }},
	}
	for _, item := range attachments {
		program := coll.Programs[item.program]
		if program == nil {
			return fail(fmt.Errorf("eBPF program %q missing", item.program))
		}
		attached, err := item.attach(program)
		if err != nil {
			return fail(fmt.Errorf("attach %s: %w", item.program, err))
		}
		h.links = append(h.links, attached)
	}
	eventsMap := coll.Maps["events"]
	if eventsMap == nil {
		return fail(fmt.Errorf("eBPF map %q missing", "events"))
	}
	reader, err := ringbuf.NewReader(eventsMap)
	if err != nil {
		return fail(fmt.Errorf("open events ring buffer: %w", err))
	}
	h.reader = reader
	go h.readLoop()
	return h, nil
}

func (h *Handle) readLoop() {
	defer close(h.Events)
	defer close(h.Errors)
	for {
		record, err := h.reader.Read()
		if err != nil {
			if !errors.Is(err, ringbuf.ErrClosed) {
				h.sendError(err)
			}
			return
		}
		var raw rawEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &raw); err != nil {
			h.sendError(fmt.Errorf("decode eBPF event: %w", err))
			continue
		}
		h.Events <- convert(raw)
	}
}

func convert(raw rawEvent) Event {
	direction := "unknown"
	if raw.Direction == 1 {
		direction = "ingress"
	} else if raw.Direction == 2 {
		direction = "egress"
	}
	protocol := "unknown"
	if raw.Protocol == 6 {
		protocol = "tcp"
	}
	return Event{
		Timestamp: time.Now().UTC(), Type: raw.Type, PID: raw.PID, PPID: raw.PPID,
		UID: raw.UID, GID: raw.GID, Direction: direction, Protocol: protocol,
		SrcIP: address(raw.Family, raw.SrcAddr), SrcPort: raw.SrcPort,
		DstIP: address(raw.Family, raw.DstAddr), DstPort: raw.DstPort,
		Bytes: raw.Bytes, Packets: raw.Packets, ConnectionCount: raw.Connections,
		Comm: cString(raw.Comm[:]), Filename: cString(raw.Filename[:]),
	}
}

func address(family uint16, raw [16]byte) string {
	if family == 2 {
		return net.IP(raw[:4]).String()
	}
	if family == 10 {
		return net.IP(raw[:]).String()
	}
	return ""
}

func cString(raw []byte) string {
	if i := bytes.IndexByte(raw, 0); i >= 0 {
		raw = raw[:i]
	}
	return strings.TrimSpace(string(raw))
}

func (h *Handle) sendError(err error) {
	select {
	case h.Errors <- err:
	default:
	}
}

func (h *Handle) Close() error {
	var first error
	h.closeOnce.Do(func() {
		if h.reader != nil {
			first = h.reader.Close()
		}
		for _, attached := range h.links {
			if err := attached.Close(); err != nil && first == nil {
				first = err
			}
		}
		if h.collection != nil {
			h.collection.Close()
		}
	})
	return first
}
