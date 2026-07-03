package service

import (
	"testing"
	"time"

	"github.com/vmlens/vmlens/backend/internal/model"
)

func TestVisibleVMDefaultGraphRequiresRecentHeartbeat(t *testing.T) {
	now := time.Now()
	if !visibleVM(model.VM{Status: "online", LastSeen: now.Add(-30 * time.Second)}, "", now) {
		t.Fatal("recent online VM should be visible")
	}
	if visibleVM(model.VM{Status: "online", LastSeen: now.Add(-2 * time.Minute)}, "", now) {
		t.Fatal("old heartbeat must not leave a ghost node")
	}
	if visibleVM(model.VM{Status: "offline", LastSeen: now}, "", now) {
		t.Fatal("offline VM should be hidden from live graph")
	}
}

func TestVisibleVMExplicitStatusSupportsAudit(t *testing.T) {
	now := time.Now()
	vm := model.VM{Status: "offline", LastSeen: now.Add(-time.Hour)}
	if !visibleVM(vm, "offline", now) {
		t.Fatal("explicit offline query should return offline VM")
	}
}
