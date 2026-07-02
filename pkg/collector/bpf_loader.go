package collector

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/vmlens/vmlens-ebpf/pkg/model"
	"github.com/vmlens/vmlens-ebpf/pkg/privacy"
)

// BPFHandles owns loaded CO-RE collections and their tracepoint/kprobe links.
// When these probes are available their ring buffers are the process/network
// event source; /proc is still used to enrich exec records and sample resources.
type BPFHandles struct {
	collections   []*ebpf.Collection
	links         []link.Link
	readers       []*ringbuf.Reader
	ProcessEvents chan model.ProcessEvent
	NetworkEvents chan model.NetworkFlow
	Errors        chan error
	mu            sync.Mutex
	known         map[int]model.ProcessEvent
}

type rawBPFEvent struct {
	TimestampNS               uint64
	Type, PID, PPID, UID, GID uint32
	Family, DstPort           uint16
	DstAddr                   [16]byte
	Comm                      [16]byte
	Filename                  [256]byte
}

func LoadBPF(dir string) (*BPFHandles, error) {
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("remove BPF memlock limit: %w", err)
	}
	h := &BPFHandles{ProcessEvents: make(chan model.ProcessEvent, 1024), NetworkEvents: make(chan model.NetworkFlow, 256), Errors: make(chan error, 8), known: map[int]model.ProcessEvent{}}
	loaded := 0
	objects := []struct {
		file     string
		programs map[string][3]string
	}{
		{"execwatch.bpf.o", map[string][3]string{"trace_exec": {"tracepoint", "syscalls", "sys_enter_execve"}, "trace_exit": {"tracepoint", "sched", "sched_process_exit"}}},
		{"netwatch.bpf.o", map[string][3]string{"tcp_v4_connect": {"kprobe", "tcp_v4_connect", ""}}},
		{"resourcewatch.bpf.o", map[string][3]string{"count_switch": {"tracepoint", "sched", "sched_switch"}}},
	}
	for _, obj := range objects {
		path := filepath.Join(dir, obj.file)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		spec, err := ebpf.LoadCollectionSpec(path)
		if err != nil {
			h.Close()
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		coll, err := ebpf.NewCollection(spec)
		if err != nil {
			h.Close()
			return nil, fmt.Errorf("load %s: %w", path, err)
		}
		h.collections = append(h.collections, coll)
		for _, mapName := range []string{"events", "net_events"} {
			if m := coll.Maps[mapName]; m != nil {
				reader, readErr := ringbuf.NewReader(m)
				if readErr != nil {
					h.Close()
					return nil, fmt.Errorf("open %s ring buffer: %w", mapName, readErr)
				}
				h.readers = append(h.readers, reader)
				go h.readLoop(reader, mapName)
			}
		}
		for name, target := range obj.programs {
			prog := coll.Programs[name]
			if prog == nil {
				h.Close()
				return nil, fmt.Errorf("program %s missing in %s", name, path)
			}
			var l link.Link
			if target[0] == "kprobe" {
				l, err = link.Kprobe(target[1], prog, nil)
			} else {
				l, err = link.Tracepoint(target[1], target[2], prog, nil)
			}
			if err != nil {
				h.Close()
				return nil, fmt.Errorf("attach %s: %w", name, err)
			}
			h.links = append(h.links, l)
		}
		loaded++
	}
	if loaded == 0 {
		return nil, fmt.Errorf("no eBPF objects found in %s; using /proc fallback", dir)
	}
	return h, nil
}

func (h *BPFHandles) readLoop(reader *ringbuf.Reader, mapName string) {
	for {
		record, err := reader.Read()
		if err != nil {
			if err != ringbuf.ErrClosed {
				select {
				case h.Errors <- err:
				default:
				}
			}
			return
		}
		var raw rawBPFEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &raw); err != nil {
			select {
			case h.Errors <- err:
			default:
			}
			continue
		}
		now := time.Now()
		comm := cstring(raw.Comm[:])
		switch raw.Type {
		case 1:
			time.Sleep(2 * time.Millisecond)
			e, err := readProcess(int(raw.PID))
			if err != nil {
				e = model.ProcessEvent{PID: int(raw.PID), UID: raw.UID, GID: raw.GID, Process: comm, Executable: cstring(raw.Filename[:]), ArgvSanitized: []string{cstring(raw.Filename[:])}, Command: cstring(raw.Filename[:])}
			}
			e.ArgvSanitized = privacy.SanitizeArgs(e.ArgvSanitized)
			e.Command = strings.Join(e.ArgvSanitized, " ")
			e.EventType = "process_exec"
			e.Timestamp = now
			e.StartTime = now
			h.mu.Lock()
			h.known[e.PID] = e
			h.mu.Unlock()
			h.ProcessEvents <- e
		case 2:
			h.mu.Lock()
			e := h.known[int(raw.PID)]
			delete(h.known, int(raw.PID))
			h.mu.Unlock()
			e.EventType = "process_exit"
			e.PID = int(raw.PID)
			e.Process = comm
			e.Timestamp = now
			e.EndTime = &now
			h.ProcessEvents <- e
		case 3:
			ip := net.IP(raw.DstAddr[:4]).String()
			h.NetworkEvents <- model.NetworkFlow{EventType: "network_flow", Timestamp: now, PID: int(raw.PID), Process: comm, DstIP: ip, DstPort: int(raw.DstPort), Protocol: "tcp"}
		}
	}
}
func cstring(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return strings.TrimSpace(string(b))
}
func (h *BPFHandles) Close() error {
	var first error
	for _, r := range h.readers {
		if err := r.Close(); err != nil && first == nil {
			first = err
		}
	}
	for _, l := range h.links {
		if err := l.Close(); err != nil && first == nil {
			first = err
		}
	}
	for _, c := range h.collections {
		c.Close()
	}
	h.links = nil
	h.collections = nil
	return first
}
