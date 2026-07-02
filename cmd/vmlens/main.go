package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/vmlens/vmlens-ebpf/pkg/agent"
	"github.com/vmlens/vmlens-ebpf/pkg/config"
	jsonlog "github.com/vmlens/vmlens-ebpf/pkg/logger"
	"github.com/vmlens/vmlens-ebpf/pkg/model"
)

var version = "dev"

func main() {
	if err := execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "vmlens:", err)
		os.Exit(1)
	}
}
func execute(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "version":
		fmt.Println("vmlens", version)
		return nil
	case "run":
		return run(args[1:])
	case "ssh":
		return ssh(args[1:])
	case "top":
		return top(args[1:])
	case "processes":
		return processes(args[1:])
	case "help", "--help", "-h":
		return usage()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
func usage() error {
	fmt.Print(`VMLens: eBPF-Based SSH Session and Resource Usage Monitor for Linux VMs

Usage:
  vmlens run [--config path]
  vmlens ssh sessions|watch|inspect <session_id> [--config path]
  vmlens top [--by cpu|memory|network|disk] [--session id] [--config path]
  vmlens processes [--config path]
  vmlens version
`)
	return nil
}
func loadFS(name string, args []string) (config.Config, *flag.FlagSet, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	path := fs.String("config", "/etc/vmlens/vmlens.yaml", "configuration file")
	if err := fs.Parse(args); err != nil {
		return config.Config{}, fs, err
	}
	c, err := config.Load(*path)
	return c, fs, err
}
func run(args []string) error {
	c, _, err := loadFS("run", args)
	if err != nil {
		return err
	}
	a, err := agent.New(c)
	if err != nil {
		return err
	}
	defer a.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	fmt.Printf("VMLens running; metrics=http://%s/metrics logs=%s\n", c.Server.ListenAddr, c.Logging.Dir)
	return a.Run(ctx)
}

func ssh(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: vmlens ssh sessions|watch|inspect")
	}
	sub := args[0]
	rest := args[1:]
	if sub == "inspect" {
		if len(rest) == 0 {
			return errors.New("session id required")
		}
		id := rest[0]
		c, _, err := loadFS("ssh inspect", rest[1:])
		if err != nil {
			return err
		}
		return inspect(c, id)
	}
	c, _, err := loadFS("ssh "+sub, rest)
	if err != nil {
		return err
	}
	switch sub {
	case "sessions":
		return sessions(c)
	case "watch":
		return watch(c)
	default:
		return fmt.Errorf("unknown ssh command %q", sub)
	}
}
func sessionState(path string) (map[string]model.SSHSession, error) {
	events, err := jsonlog.ReadJSONLines[model.SSHSession](path)
	if err != nil {
		return nil, err
	}
	out := map[string]model.SSHSession{}
	pidToID := map[int]string{}
	for _, e := range events {
		if e.EventType == "ssh_login" {
			out[e.SessionID] = e
			pidToID[e.SSHPID] = e.SessionID
		} else if e.EventType == "ssh_session_update" {
			out[e.SessionID] = e
			pidToID[e.SSHPID] = e.SessionID
		} else if e.EventType == "ssh_logout" {
			id := e.SessionID
			if id == "" {
				id = pidToID[e.SSHPID]
			}
			if s, ok := out[id]; ok {
				s.Status = "closed"
				s.EndTime = e.EndTime
				out[id] = s
			}
		}
	}
	return out, nil
}
func sessions(c config.Config) error {
	states, err := sessionState(c.Logging.SSHLogPath)
	if err != nil {
		return err
	}
	list := make([]model.SSHSession, 0, len(states))
	for _, s := range states {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].StartTime.After(list[j].StartTime) })
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SESSION ID\tUSER\tREMOTE IP\tTTY\tSTARTED\tSTATUS")
	for _, s := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", s.SessionID, s.User, s.RemoteIP, s.TTY, s.StartTime.Format("2006-01-02 15:04"), s.Status)
	}
	return w.Flush()
}

func inspect(c config.Config, id string) error {
	states, err := sessionState(c.Logging.SSHLogPath)
	if err != nil {
		return err
	}
	s, ok := states[id]
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}
	ps, _ := jsonlog.ReadJSONLines[model.ProcessEvent](c.Logging.ProcessLogPath)
	rs, _ := jsonlog.ReadJSONLines[model.ResourceEvent](c.Logging.ResourceLogPath)
	ns, _ := jsonlog.ReadJSONLines[model.NetworkFlow](c.Logging.NetworkLogPath)
	as, _ := jsonlog.ReadJSONLines[model.AnalysisSummary](c.Logging.AnalysisLogPath)
	fmt.Printf("Session: %s\nUser: %s\nRemote IP: %s\nTTY: %s\nStatus: %s\nLogin: %s\n", id, s.User, s.RemoteIP, s.TTY, s.Status, s.StartTime.Format(time.RFC3339))
	if s.EndTime != nil {
		fmt.Println("Logout:", s.EndTime.Format(time.RFC3339))
	}
	var topCPU, topMem model.ResourceEvent
	var topNet model.NetworkFlow
	var cmds []model.ProcessEvent
	for _, p := range ps {
		if p.SessionID == id && p.EventType == "process_exec" {
			cmds = append(cmds, p)
		}
	}
	for _, r := range rs {
		if r.SessionID != id {
			continue
		}
		if r.CPUPercent > topCPU.CPUPercent {
			topCPU = r
		}
		if r.RSSBytes > topMem.RSSBytes {
			topMem = r
		}
	}
	for _, n := range ns {
		if n.SessionID == id && n.RXBytes+n.TXBytes > topNet.RXBytes+topNet.TXBytes {
			topNet = n
		}
	}
	fmt.Println("\nTop resource usage:")
	fmt.Printf("- CPU: %s PID=%d %.1f%%\n", topCPU.Process, topCPU.PID, topCPU.CPUPercent)
	fmt.Printf("- Memory: %s PID=%d rss=%s\n", topMem.Process, topMem.PID, bytes(topMem.RSSBytes))
	fmt.Printf("- Network: %s PID=%d rx=%s tx=%s (fallback counters may be zero)\n", topNet.Process, topNet.PID, bytes(topNet.RXBytes), bytes(topNet.TXBytes))
	fmt.Println("\nObserved commands:")
	for _, p := range cmds {
		fmt.Printf("%s %s\n", p.Timestamp.Format("15:04:05"), p.Command)
	}
	fmt.Println("\nHeavy activity summary:")
	found := false
	for _, a := range as {
		if a.SessionID == id {
			found = true
			fmt.Printf("- [%s] %s\n", a.Reason, a.Summary)
		}
	}
	if !found {
		fmt.Println("- No threshold violations observed.")
	}
	return nil
}

func top(args []string) error {
	fs := flag.NewFlagSet("top", flag.ContinueOnError)
	path := fs.String("config", "/etc/vmlens/vmlens.yaml", "configuration file")
	by := fs.String("by", "cpu", "cpu|memory|network|disk")
	sid := fs.String("session", "", "session id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := config.Load(*path)
	if err != nil {
		return err
	}
	if *by == "network" {
		flows, _ := jsonlog.ReadJSONLines[model.NetworkFlow](c.Logging.NetworkLogPath)
		sort.Slice(flows, func(i, j int) bool { return flows[i].RXBytes+flows[i].TXBytes > flows[j].RXBytes+flows[j].TXBytes })
		fmt.Println("PID\tPROCESS\tRX\tTX\tDESTINATION")
		n := 0
		for _, f := range flows {
			if *sid != "" && f.SessionID != *sid {
				continue
			}
			fmt.Printf("%d\t%s\t%s\t%s\t%s:%d\n", f.PID, f.Process, bytes(f.RXBytes), bytes(f.TXBytes), f.DstIP, f.DstPort)
			n++
			if n == 15 {
				break
			}
		}
		return nil
	}
	rows, _ := jsonlog.ReadJSONLines[model.ResourceEvent](c.Logging.ResourceLogPath)
	latest := map[int]model.ResourceEvent{}
	for _, r := range rows {
		if *sid == "" || r.SessionID == *sid {
			latest[r.PID] = r
		}
	}
	rows = rows[:0]
	for _, r := range latest {
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		switch *by {
		case "memory":
			return rows[i].RSSBytes > rows[j].RSSBytes
		case "disk":
			return rows[i].DiskReadBytes+rows[i].DiskWriteBytes > rows[j].DiskReadBytes+rows[j].DiskWriteBytes
		default:
			return rows[i].CPUPercent > rows[j].CPUPercent
		}
	})
	fmt.Println("PID\tPROCESS\tCPU\tRSS\tDISK READ\tDISK WRITE\tSESSION")
	for i, r := range rows {
		if i == 15 {
			break
		}
		fmt.Printf("%d\t%s\t%.1f%%\t%s\t%s\t%s\t%s\n", r.PID, r.Process, r.CPUPercent, bytes(r.RSSBytes), bytes(r.DiskReadBytes), bytes(r.DiskWriteBytes), r.SessionID)
	}
	return nil
}

func processes(args []string) error {
	c, _, err := loadFS("processes", args)
	if err != nil {
		return err
	}
	events, err := jsonlog.ReadJSONLines[model.ProcessEvent](c.Logging.ProcessLogPath)
	if err != nil {
		return err
	}
	active := map[int]model.ProcessEvent{}
	for _, e := range events {
		if e.EventType == "process_exec" {
			active[e.PID] = e
		} else {
			delete(active, e.PID)
		}
	}
	list := make([]model.ProcessEvent, 0, len(active))
	for _, p := range active {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].PID < list[j].PID })
	fmt.Println("PID\tPPID\tUSER\tPROCESS\tSESSION\tCOMMAND")
	for _, p := range list {
		fmt.Printf("%d\t%d\t%s\t%s\t%s\t%s\n", p.PID, p.PPID, p.User, p.Process, p.SessionID, p.Command)
	}
	return nil
}

func watch(c config.Config) error {
	paths := []string{c.Logging.SSHLogPath, c.Logging.ProcessLogPath, c.Logging.AnalysisLogPath}
	type source struct {
		f *os.File
		r *bufio.Reader
	}
	var sources []source
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		_, _ = f.Seek(0, io.SeekEnd)
		sources = append(sources, source{f, bufio.NewReader(f)})
	}
	if len(sources) == 0 {
		return fmt.Errorf("no VMLens logs found in %s", c.Logging.Dir)
	}
	defer func() {
		for _, s := range sources {
			s.f.Close()
		}
	}()
	for {
		for i := range sources {
			line, err := sources[i].r.ReadString('\n')
			if err == nil {
				printWatch(line)
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
}
func printWatch(line string) {
	var h struct {
		EventType string    `json:"event_type"`
		Timestamp time.Time `json:"timestamp"`
	}
	if json.Unmarshal([]byte(line), &h) != nil {
		return
	}
	switch h.EventType {
	case "ssh_login", "ssh_logout":
		var s model.SSHSession
		_ = json.Unmarshal([]byte(line), &s)
		fmt.Printf("[%s] %s user=%s remote=%s tty=%s session=%s\n", h.Timestamp.Format("15:04:05"), strings.ReplaceAll(h.EventType, "_", " "), s.User, s.RemoteIP, s.TTY, s.SessionID)
	case "process_exec":
		var p model.ProcessEvent
		_ = json.Unmarshal([]byte(line), &p)
		if p.SessionID != "" {
			fmt.Printf("[%s] session=%s exec pid=%d cmd=%q\n", h.Timestamp.Format("15:04:05"), p.SessionID, p.PID, p.Command)
		}
	case "analysis_summary":
		var a model.AnalysisSummary
		_ = json.Unmarshal([]byte(line), &a)
		fmt.Printf("[%s] session=%s %s pid=%d process=%s %s\n", h.Timestamp.Format("15:04:05"), a.SessionID, a.Reason, a.PID, a.TopProcess, a.Summary)
	}
}
func bytes(v uint64) string {
	const unit = 1024
	if v < unit {
		return strconv.FormatUint(v, 10) + "B"
	}
	div, exp := uint64(unit), 0
	for n := v / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(v)/float64(div), "KMGTPE"[exp])
}
