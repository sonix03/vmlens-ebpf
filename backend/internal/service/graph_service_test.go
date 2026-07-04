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
}
