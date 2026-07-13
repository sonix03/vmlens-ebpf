package correlator

import (
	"sync"

	"github.com/vmlens/vmlens-ebpf/pkg/model"
)

// SessionCorrelator attributes descendants of an sshd process to a session.
// PID ancestry is retained after exit so late resource/network events can still
// be attributed. A process started independently is never assigned by UID alone.
type SessionCorrelator struct {
	mu         sync.RWMutex
	sessions   map[string]model.SSHSession
	pidSession map[int]string
	parents    map[int]int
}

func New() *SessionCorrelator {
	return &SessionCorrelator{sessions: map[string]model.SSHSession{}, pidSession: map[int]string{}, parents: map[int]int{}}
}

func (c *SessionCorrelator) UpsertSession(s model.SSHSession) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions[s.SessionID] = s
	if s.SSHPID > 0 {
		c.pidSession[s.SSHPID] = s.SessionID
	}
}

func (c *SessionCorrelator) AttributeProcess(e *model.ProcessEvent) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.parents[e.PID] = e.PPID
	if sid := c.pidSession[e.PID]; sid != "" {
		e.SessionID = sid
		return sid
	}
	for pid, n := e.PPID, 0; pid > 1 && n < 64; n++ {
		if sid := c.pidSession[pid]; sid != "" {
			c.pidSession[e.PID] = sid
			e.SessionID = sid
			return sid
		}
		next, ok := c.parents[pid]
		if !ok || next == pid {
			break
		}
		pid = next
	}
	return ""
}

func (c *SessionCorrelator) SessionForPID(pid int) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pidSession[pid]
}
func (c *SessionCorrelator) SetTTY(id, tty string) (model.SSHSession, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.sessions[id]
	if !ok || s.TTY != "" || tty == "" {
		return s, false
	}
	s.TTY = tty
	c.sessions[id] = s
	return s, true
}
func (c *SessionCorrelator) Session(id string) (model.SSHSession, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.sessions[id]
	return s, ok
}
func (c *SessionCorrelator) Sessions() []model.SSHSession {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.SSHSession, 0, len(c.sessions))
	for _, s := range c.sessions {
		out = append(out, s)
	}
	return out
}
