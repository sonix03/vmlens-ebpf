package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vmlens/vmlens-ebpf/internal/classifier"
	"github.com/vmlens/vmlens-ebpf/internal/config"
	vebpf "github.com/vmlens/vmlens-ebpf/internal/ebpf"
	"github.com/vmlens/vmlens-ebpf/internal/metadata"
)

type Event struct {
	Timestamp       time.Time  `json:"timestamp"`
	Kind            string     `json:"kind"`
	TenantID        string     `json:"tenant_id,omitempty"`
	UserID          string     `json:"user_id,omitempty"`
	VMID            string     `json:"vm_id"`
	Hostname        string     `json:"hostname"`
	Direction       string     `json:"direction,omitempty"`
	Protocol        string     `json:"protocol,omitempty"`
	SrcIP           string     `json:"src_ip,omitempty"`
	SrcPort         uint16     `json:"src_port,omitempty"`
	DstIP           string     `json:"dst_ip,omitempty"`
	DstPort         uint16     `json:"dst_port,omitempty"`
	Scope           string     `json:"scope,omitempty"`
	BytesSent       uint64     `json:"bytes_sent"`
	BytesReceived   uint64     `json:"bytes_received"`
	PacketsSent     uint64     `json:"packets_sent"`
	PacketsReceived uint64     `json:"packets_received"`
	ConnectionCount uint64     `json:"connection_count"`
	Interface       string     `json:"interface,omitempty"`
	TCPState        string     `json:"tcp_state,omitempty"`
	PortClass       string     `json:"port_class,omitempty"`
	PID             uint32     `json:"pid,omitempty"`
	PPID            uint32     `json:"ppid,omitempty"`
	UID             uint32     `json:"uid,omitempty"`
	GID             uint32     `json:"gid,omitempty"`
	Username        string     `json:"username,omitempty"`
	Process         string     `json:"process,omitempty"`
	ExePath         string     `json:"exe_path,omitempty"`
	ContainerID     string     `json:"container_id,omitempty"`
	Cgroup          string     `json:"cgroup,omitempty"`
	CommandLine     string     `json:"command_line,omitempty"`
	LoginUser       string     `json:"login_user,omitempty"`
	SSHSourceIP     string     `json:"ssh_source_ip,omitempty"`
	TTY             string     `json:"tty,omitempty"`
	SessionID       string     `json:"session_id,omitempty"`
	LoginTime       *time.Time `json:"login_time,omitempty"`
	ParentProcess   string     `json:"parent_process,omitempty"`
	ChildProcess    string     `json:"child_process,omitempty"`
}

type Collector struct {
	classifier *classifier.Classifier
	vm         metadata.VM
	privacy    config.PrivacyConfig
}

func New(c *classifier.Classifier, vm metadata.VM, privacy config.PrivacyConfig) *Collector {
	return &Collector{classifier: c, vm: vm, privacy: privacy}
}

func (c *Collector) Run(ctx context.Context, input <-chan vebpf.Event) <-chan Event {
	out := make(chan Event, 4096)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case raw, ok := <-input:
				if !ok {
					return
				}
				event := c.convert(raw)
				select {
				case out <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}

func (c *Collector) convert(raw vebpf.Event) Event {
	e := Event{
		Timestamp: raw.Timestamp, TenantID: c.vm.TenantID, UserID: c.vm.UserID,
		VMID: c.vm.VMID, Hostname: c.vm.Hostname, PID: raw.PID, PPID: raw.PPID,
		UID: raw.UID, GID: raw.GID, Process: raw.Comm, ExePath: raw.Filename,
	}
	if raw.Type == vebpf.EventExec {
		e.Kind = "process_exec"
	} else {
		e.Kind = "network"
		e.Direction, e.Protocol = raw.Direction, raw.Protocol
		e.SrcIP, e.SrcPort, e.DstIP, e.DstPort = raw.SrcIP, raw.SrcPort, raw.DstIP, raw.DstPort
		e.ConnectionCount = uint64(raw.ConnectionCount)
		if raw.Direction == "egress" {
			e.BytesSent, e.PacketsSent = raw.Bytes, uint64(raw.Packets)
			e.Scope = c.classifier.Scope(raw.DstIP)
			e.Interface = c.vm.InterfaceForIP(raw.SrcIP)
		} else {
			e.BytesReceived, e.PacketsReceived = raw.Bytes, uint64(raw.Packets)
			e.Scope = c.classifier.Scope(raw.SrcIP)
			e.Interface = c.vm.InterfaceForIP(raw.DstIP)
		}
		e.PortClass = classifier.PortClass(raw.DstPort)
		switch raw.Type {
		case vebpf.EventConnect:
			e.TCPState = "SYN_SENT"
		case vebpf.EventAccept, vebpf.EventSend, vebpf.EventReceive:
			e.TCPState = "ESTABLISHED"
		}
	}
	c.enrichProcess(&e)
	if e.Kind == "network" && e.Direction == "ingress" && e.DstPort == 22 && e.Process == "sshd" {
		e.SSHSourceIP = e.SrcIP
		e.SessionID = fmt.Sprintf("ssh-%d-%d", e.PID, e.Timestamp.UnixNano())
		loginTime := e.Timestamp
		e.LoginTime = &loginTime
		e.ChildProcess = e.Process
	}
	return e
}

var containerID = regexp.MustCompile(`(?:^|[-/])([a-f0-9]{64})(?:\.scope)?(?:$|/)`)

func (c *Collector) enrichProcess(e *Event) {
	pid := strconv.FormatUint(uint64(e.PID), 10)
	base := filepath.Join("/proc", pid)
	if u, err := user.LookupId(strconv.FormatUint(uint64(e.UID), 10)); err == nil {
		e.Username = u.Username
	}
	if exe, err := os.Readlink(filepath.Join(base, "exe")); err == nil {
		e.ExePath = exe
	}
	if e.Process == "" {
		e.Process = readTrimmed(filepath.Join(base, "comm"))
	}
	e.Cgroup = readTrimmed(filepath.Join(base, "cgroup"))
	if match := containerID.FindStringSubmatch(e.Cgroup); len(match) == 2 {
		e.ContainerID = match[1]
	}
	if target, err := os.Readlink(filepath.Join(base, "fd/0")); err == nil && strings.Contains(target, "/dev/pts/") {
		e.TTY = strings.TrimPrefix(target, "/dev/")
	}
	if e.PPID > 0 {
		e.ParentProcess = readTrimmed(filepath.Join("/proc", strconv.FormatUint(uint64(e.PPID), 10), "comm"))
	}
	if c.privacy.CollectCmdline {
		if raw, err := os.ReadFile(filepath.Join(base, "cmdline")); err == nil {
			args := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
			if c.privacy.RedactSecrets {
				args = redactArgs(args)
			}
			e.CommandLine = strings.Join(args, " ")
		}
	}
}

func readTrimmed(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	var lines []string
	for s.Scan() {
		lines = append(lines, strings.TrimSpace(s.Text()))
	}
	return strings.Join(lines, ";")
}

var secretKey = regexp.MustCompile(`(?i)(password|passwd|token|secret|api[_-]?key|authorization|cookie|credential)`)

func redactArgs(args []string) []string {
	out := append([]string(nil), args...)
	redactNext := false
	for i, arg := range out {
		if redactNext {
			out[i], redactNext = "[REDACTED]", false
			continue
		}
		key, _, assignment := strings.Cut(arg, "=")
		if assignment && secretKey.MatchString(strings.TrimLeft(key, "-")) {
			out[i] = key + "=[REDACTED]"
			continue
		}
		if strings.HasPrefix(arg, "-") && secretKey.MatchString(strings.TrimLeft(arg, "-")) {
			redactNext = true
		}
	}
	return out
}
