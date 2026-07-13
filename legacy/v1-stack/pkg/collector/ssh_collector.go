package collector

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vmlens/vmlens-ebpf/pkg/config"
	"github.com/vmlens/vmlens-ebpf/pkg/model"
)

type SSHCollector struct {
	cfg    config.Config
	Events chan model.SSHSession
	Errors chan error
}

var acceptedRE = regexp.MustCompile(`sshd\[([0-9]+)\]: Accepted ([^ ]+) for ([^ ]+) from ([^ ]+) port ([0-9]+)`)
var disconnectRE = regexp.MustCompile(`sshd\[([0-9]+)\]: (?:Disconnected from user ([^ ]+) |Received disconnect from ([^ ]+)|pam_unix\(sshd:session\): session closed for user ([^ ]+))`)

func NewSSH(cfg config.Config) *SSHCollector {
	return &SSHCollector{cfg: cfg, Events: make(chan model.SSHSession, 64), Errors: make(chan error, 4)}
}

func (c *SSHCollector) Run(ctx context.Context) {
	defer close(c.Events)
	if !c.cfg.SSH.Enabled {
		<-ctx.Done()
		return
	}
	if c.cfg.SSH.ParseAuthLog {
		if f, err := os.Open(c.cfg.SSH.AuthLogPath); err == nil {
			defer f.Close()
			c.follow(ctx, f)
			return
		}
	}
	if c.cfg.SSH.ParseJournald {
		cmd := exec.CommandContext(ctx, "journalctl", "-f", "-n", "0", "-u", "ssh.service", "-u", "sshd.service", "-o", "short-iso")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			c.sendErr(err)
			return
		}
		if err := cmd.Start(); err != nil {
			c.sendErr(fmt.Errorf("start journalctl: %w", err))
			return
		}
		c.scan(ctx, stdout)
		_ = cmd.Wait()
		return
	}
	c.sendErr(fmt.Errorf("no SSH log source available"))
	<-ctx.Done()
}

func (c *SSHCollector) follow(ctx context.Context, f *os.File) {
	_, _ = f.Seek(0, io.SeekEnd)
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if err == nil {
			c.parse(line)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}
func (c *SSHCollector) scan(ctx context.Context, r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			c.parse(s.Text())
		}
	}
	if err := s.Err(); err != nil {
		c.sendErr(err)
	}
}

func (c *SSHCollector) parse(line string) {
	now := time.Now()
	if m := acceptedRE.FindStringSubmatch(line); len(m) > 0 {
		pid, _ := strconv.Atoi(m[1])
		port, _ := strconv.Atoi(m[5])
		sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%s:%d", pid, m[4], now.UnixNano())))
		s := model.SSHSession{EventType: "ssh_login", Timestamp: now, SessionID: fmt.Sprintf("ssh-%d-%x", pid, sum[:3]), User: m[3], RemoteIP: m[4], RemotePort: port, AuthMethod: m[2], SSHPID: pid, StartTime: now, Status: "active"}
		c.Events <- s
		return
	}
	if m := disconnectRE.FindStringSubmatch(line); len(m) > 0 {
		pid, _ := strconv.Atoi(m[1])
		user := ""
		for _, x := range m[2:] {
			if x != "" && !strings.Contains(x, ".") && !strings.Contains(x, ":") {
				user = x
			}
		}
		c.Events <- model.SSHSession{EventType: "ssh_logout", Timestamp: now, User: user, SSHPID: pid, EndTime: &now, Status: "closed"}
	}
}
func (c *SSHCollector) sendErr(err error) {
	select {
	case c.Errors <- err:
	default:
	}
}
