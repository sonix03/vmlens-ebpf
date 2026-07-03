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
