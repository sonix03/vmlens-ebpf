package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vmlens/vmlens-ebpf/pkg/model"
	"github.com/vmlens/vmlens-ebpf/pkg/privacy"
)

// ExecCollector is the portable /proc fallback used when generated eBPF objects
// are unavailable. It detects process starts/exits at the configured interval.
type ExecCollector struct {
	Interval time.Duration
	Sanitize bool
	Events   chan model.ProcessEvent
	known    map[int]model.ProcessEvent
}

func NewExec(interval time.Duration, sanitize bool) *ExecCollector {
	return &ExecCollector{Interval: interval, Sanitize: sanitize, Events: make(chan model.ProcessEvent, 1024), known: map[int]model.ProcessEvent{}}
}
func (c *ExecCollector) Run(ctx context.Context) {
	defer close(c.Events)
	c.sample()
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
func (c *ExecCollector) sample() {
	entries, _ := os.ReadDir("/proc")
	current := map[int]bool{}
	now := time.Now()
	for _, d := range entries {
		pid, err := strconv.Atoi(d.Name())
		if err != nil {
			continue
		}
		current[pid] = true
		if _, ok := c.known[pid]; ok {
			continue
		}
		e, err := readProcess(pid)
		if err != nil {
			continue
		}
		e.EventType = "process_exec"
		e.Timestamp = now
		e.StartTime = now
		if c.Sanitize {
			e.ArgvSanitized = privacy.SanitizeArgs(e.ArgvSanitized)
			e.Command = strings.Join(e.ArgvSanitized, " ")
		}
		c.known[pid] = e
		c.Events <- e
	}
	for pid, e := range c.known {
		if !current[pid] {
			end := now
			e.EventType = "process_exit"
			e.Timestamp = now
			e.EndTime = &end
			c.Events <- e
			delete(c.known, pid)
		}
	}
}
func readProcess(pid int) (model.ProcessEvent, error) {
	base := fmt.Sprintf("/proc/%d", pid)
	stat, err := os.ReadFile(filepath.Join(base, "stat"))
	if err != nil {
		return model.ProcessEvent{}, err
	}
	s := string(stat)
	closeIdx := strings.LastIndex(s, ")")
	if closeIdx < 0 {
		return model.ProcessEvent{}, fmt.Errorf("bad stat")
	}
	before := strings.Index(s, "(")
	comm := s[before+1 : closeIdx]
	fields := strings.Fields(s[closeIdx+2:])
	if len(fields) < 2 {
		return model.ProcessEvent{}, fmt.Errorf("short stat")
	}
	ppid, _ := strconv.Atoi(fields[1])
	status, _ := os.Open(filepath.Join(base, "status"))
	var uid, gid uint64
	if status != nil {
		sc := bufio.NewScanner(status)
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			if len(f) > 1 && f[0] == "Uid:" {
				uid, _ = strconv.ParseUint(f[1], 10, 32)
			}
			if len(f) > 1 && f[0] == "Gid:" {
				gid, _ = strconv.ParseUint(f[1], 10, 32)
			}
		}
		status.Close()
	}
	cmdRaw, _ := os.ReadFile(filepath.Join(base, "cmdline"))
	args := strings.Split(strings.TrimRight(string(cmdRaw), "\x00"), "\x00")
	if len(args) == 1 && args[0] == "" {
		args = []string{comm}
	}
	exe, _ := os.Readlink(filepath.Join(base, "exe"))
	username := ""
	if u, err := user.LookupId(strconv.FormatUint(uid, 10)); err == nil {
		username = u.Username
	}
	tty := ""
	if target, err := os.Readlink(filepath.Join(base, "fd/0")); err == nil && strings.Contains(target, "/dev/pts/") {
		tty = strings.TrimPrefix(target, "/dev/")
	}
	return model.ProcessEvent{PID: pid, PPID: ppid, UID: uint32(uid), GID: uint32(gid), User: username, TTY: tty, Process: comm, Executable: exe, ArgvSanitized: args, Command: strings.Join(args, " ")}, nil
}
