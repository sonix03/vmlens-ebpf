package collector

import (
	"context"
	"time"

	vebpf "github.com/vmlens/vmlens-ebpf/internal/ebpf"
	"github.com/vmlens/vmlens-ebpf/internal/metadata"
)

type demoScenario struct {
	pid       uint32
	process   string
	exe       string
	direction string
	srcIP     string
	srcPort   uint16
	dstIP     string
	dstPort   uint16
	bytes     uint64
}

func Demo(ctx context.Context, vm metadata.VM) <-chan vebpf.Event {
	out := make(chan vebpf.Event, 64)
	privateIP := vm.PrivateIP
	if privateIP == "" {
		privateIP = "10.0.1.20"
	}
	scenarios := []demoScenario{
		{31001, "curl", "/usr/bin/curl", "egress", privateIP, 40110, "151.101.1.69", 443, 6 << 20},
		{31002, "dockerd", "/usr/bin/dockerd", "egress", privateIP, 40111, "34.194.164.123", 443, 12 << 20},
		{31003, "apt", "/usr/bin/apt", "egress", privateIP, 40112, "91.189.91.81", 80, 4 << 20},
		{31004, "sshd", "/usr/sbin/sshd", "ingress", "198.51.100.50", 53022, privateIP, 22, 96 << 10},
		{31005, "postgres", "/usr/bin/postgres", "egress", privateIP, 40113, "10.0.1.30", 5432, 2 << 20},
	}
	go func() {
		defer close(out)
		now := time.Now().UTC()
		for _, scenario := range scenarios {
			out <- vebpf.Event{Timestamp: now, Type: vebpf.EventExec, PID: scenario.pid, PPID: 1, UID: 1000, GID: 1000, Comm: scenario.process, Filename: scenario.exe}
		}
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		idx := 0
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				s := scenarios[idx%len(scenarios)]
				idx++
				packets := uint32(s.bytes/1400 + 1)
				eventType := uint32(vebpf.EventConnect)
				if s.direction == "ingress" {
					eventType = vebpf.EventAccept
				}
				out <- vebpf.Event{
					Timestamp: now.UTC(), Type: eventType, PID: s.pid, PPID: 1,
					UID: 1000, GID: 1000, Direction: s.direction, Protocol: "tcp",
					SrcIP: s.srcIP, SrcPort: s.srcPort, DstIP: s.dstIP, DstPort: s.dstPort,
					Bytes: s.bytes, Packets: packets, ConnectionCount: 1, Comm: s.process,
					Filename: s.exe,
				}
			}
		}
	}()
	return out
}
