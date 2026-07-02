package collector

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vmlens/vmlens-ebpf/pkg/model"
)

// NetCollector maps socket inodes from /proc/net/tcp{,6} back to process FDs.
// It emits connection metadata only; byte counters remain zero in fallback mode.
type NetCollector struct {
	Interval time.Duration
	PIDs     func() []int
	Events   chan model.NetworkFlow
	seen     map[string]bool
}

func NewNet(interval time.Duration, pids func() []int) *NetCollector {
	return &NetCollector{Interval: interval, PIDs: pids, Events: make(chan model.NetworkFlow, 256), seen: map[string]bool{}}
}

func (c *NetCollector) Run(ctx context.Context) {
	defer close(c.Events)
	t := time.NewTicker(c.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.sample()
		}
	}
}

type socketInfo struct {
	src, dst     string
	sport, dport int
	proto        string
}

func (c *NetCollector) sample() {
	sockets := readSockets()
	now := time.Now()
	for _, pid := range c.PIDs() {
		fds, _ := filepath.Glob(fmt.Sprintf("/proc/%d/fd/*", pid))
		for _, fd := range fds {
			target, err := os.Readlink(fd)
			if err != nil || !strings.HasPrefix(target, "socket:[") {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
			si, ok := sockets[inode]
			if !ok {
				continue
			}
			key := fmt.Sprintf("%d:%s", pid, inode)
			if c.seen[key] {
				continue
			}
			c.seen[key] = true
			comm, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
			c.Events <- model.NetworkFlow{EventType: "network_flow", Timestamp: now, PID: pid, Process: strings.TrimSpace(string(comm)), SrcIP: si.src, DstIP: si.dst, SrcPort: si.sport, DstPort: si.dport, Protocol: si.proto}
		}
	}
}

func readSockets() map[string]socketInfo {
	out := map[string]socketInfo{}
	for _, spec := range []struct{ path, proto string }{{"/proc/net/tcp", "tcp"}, {"/proc/net/tcp6", "tcp6"}} {
		f, err := os.Open(spec.path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Scan() // header
		for sc.Scan() {
			x := strings.Fields(sc.Text())
			if len(x) < 10 || x[3] != "01" {
				continue
			}
			src, sp := decodeAddr(x[1])
			dst, dp := decodeAddr(x[2])
			out[x[9]] = socketInfo{src, dst, sp, dp, spec.proto}
		}
		f.Close()
	}
	return out
}

func decodeAddr(v string) (string, int) {
	p := strings.Split(v, ":")
	if len(p) != 2 {
		return "", 0
	}
	port64, _ := strconv.ParseInt(p[1], 16, 32)
	b, err := strconv.ParseUint(p[0], 16, 32)
	if err != nil || len(p[0]) != 8 {
		return "", int(port64)
	}
	ip := net.IPv4(byte(b), byte(b>>8), byte(b>>16), byte(b>>24))
	return ip.String(), int(port64)
}
