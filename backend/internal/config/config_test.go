package config

import (
	"testing"
	"time"
)

func TestVMDeleteAfter(t *testing.T) {
	t.Setenv("VM_DELETE_AFTER", "20m")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.VMDeleteAfter != 20*time.Minute {
		t.Fatalf("got %s", cfg.VMDeleteAfter)
	}
}

func TestVMDeleteAfterRejectsUnsafeWindow(t *testing.T) {
	t.Setenv("VM_DELETE_AFTER", "5m")
	if _, err := Load(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestFlowActiveWindow(t *testing.T) {
	t.Setenv("FLOW_ACTIVE_WINDOW", "5s")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FlowActiveWindow != 5*time.Second {
		t.Fatalf("got %s", cfg.FlowActiveWindow)
	}
}

func TestFlowActiveWindowRejectsInvalidWindow(t *testing.T) {
	t.Setenv("FLOW_ACTIVE_WINDOW", "100ms")
	if _, err := Load(); err == nil {
		t.Fatal("expected validation error")
	}
}
