package correlator

import (
	"testing"

	"github.com/vmlens/vmlens-ebpf/pkg/model"
)

func TestAttributesOnlyDescendants(t *testing.T) {
	c := New()
	c.UpsertSession(model.SSHSession{SessionID: "ssh-1", SSHPID: 100})
	parent := model.ProcessEvent{PID: 101, PPID: 100}
	if got := c.AttributeProcess(&parent); got != "ssh-1" {
		t.Fatalf("parent got %q", got)
	}
	child := model.ProcessEvent{PID: 102, PPID: 101}
	if got := c.AttributeProcess(&child); got != "ssh-1" {
		t.Fatalf("child got %q", got)
	}
	unrelated := model.ProcessEvent{PID: 200, PPID: 1, UID: 1000}
	if got := c.AttributeProcess(&unrelated); got != "" {
		t.Fatalf("unrelated process attributed to %q", got)
	}
}
