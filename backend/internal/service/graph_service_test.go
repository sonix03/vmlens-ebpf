package service

import (
	"testing"
	"time"

	"github.com/vmlens/vmlens/backend/internal/model"
)

func TestVisibleVMDefaultGraphKeepsOfflineInventory(t *testing.T) {
	now := time.Now()
	if !visibleVM(model.VM{Status: "online", LastSeen: now.Add(-30 * time.Second)}, "", now) {
		t.Fatal("recent online VM should be visible")
	}
	if !visibleVM(model.VM{Status: "offline", LastSeen: now.Add(-time.Hour)}, "", now) {
		t.Fatal("offline VM should remain visible as inventory")
	}
	if visibleVM(model.VM{Status: "deleted", LastSeen: now}, "", now) {
		t.Fatal("deleted VM should not be visible")
	}
}

func TestVisibleVMExplicitStatusSupportsAudit(t *testing.T) {
	now := time.Now()
	vm := model.VM{Status: "offline", LastSeen: now.Add(-time.Hour)}
	if !visibleVM(vm, "offline", now) {
		t.Fatal("explicit offline query should return offline VM")
	}
}

func TestSetEdgeActivity(t *testing.T) {
	now := time.Date(2026, time.July, 4, 12, 0, 0, 0, time.UTC)
	active := model.GraphEdge{}
	setEdgeActivity(&active, now, now.Add(-2*time.Second), 3*time.Second)
	if !active.Active || !active.ActiveUntil.Equal(now.Add(time.Second)) {
		t.Fatalf("expected active edge until %s, got active=%t until=%s", now.Add(time.Second), active.Active, active.ActiveUntil)
	}

	idle := model.GraphEdge{}
	setEdgeActivity(&idle, now, now.Add(-4*time.Second), 3*time.Second)
	if idle.Active {
		t.Fatal("expected edge to become idle after the activity window")
	}

	lastErrorAt := now.Add(-time.Second)
	failed := model.GraphEdge{LastErrorAt: &lastErrorAt}
	setEdgeActivity(&failed, now, now.Add(-10*time.Second), 3*time.Second)
	if !failed.Failed || !failed.FailedUntil.Equal(now.Add(2*time.Second)) {
		t.Fatalf("expected failed edge until %s, got failed=%t until=%s", now.Add(2*time.Second), failed.Failed, failed.FailedUntil)
	}
}

func TestGraphEdgeIDAggregatesEphemeralPorts(t *testing.T) {
	first := graphEdgeID("vm-a", "vm-b", "tcp", ScopeInternalSameTenant)
	second := graphEdgeID("vm-a", "vm-b", "tcp", ScopeInternalSameTenant)
	if first != second {
		t.Fatalf("expected repeated traffic between the same VMs to share one edge id, got %q and %q", first, second)
	}
}

func TestPreferredGraphPortKeepsServicePort(t *testing.T) {
	if got := preferredGraphPort(48112, 8081); got != 8081 {
		t.Fatalf("expected service port to replace ephemeral port, got %d", got)
	}
	if got := preferredGraphPort(8081, 48112); got != 8081 {
		t.Fatalf("expected existing service port to stay selected, got %d", got)
	}
}
