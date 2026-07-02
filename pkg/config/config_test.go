package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("server:\n  listen_addr: 127.0.0.1:9999\ncollection:\n  sample_interval: 2s\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.ListenAddr != "127.0.0.1:9999" || c.Collection.SampleInterval != 2*time.Second {
		t.Fatalf("unexpected config: %+v", c)
	}
}
func TestRejectUnsafeCapture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("privacy:\n  capture_payload: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("unsafe config was accepted")
	}
}
