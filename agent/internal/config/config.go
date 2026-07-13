package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BackendURL        string
	MockMode          bool
	HeartbeatInterval time.Duration
	FlowInterval      time.Duration
	HTTPTimeout       time.Duration
	BPFObject         string
	CaptureMode       string
	CaptureInterface  string
	AgentID           string
	Hostname          string
	MachineID         string
	TenantID          string
	PrivateIPs        []string
	PublicIP          string
	MACAddresses      []string
	IgnoreIPs         []string
	AllowCIDRs        []string
	DenyCIDRs         []string
	Environment       string
	AgentVersion      string
}

func Load() (Config, error) {
	mockMode, err := strconv.ParseBool(env("MOCK_MODE", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("parse MOCK_MODE: %w", err)
	}
	heartbeat, err := time.ParseDuration(env("HEARTBEAT_INTERVAL", "20s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse HEARTBEAT_INTERVAL: %w", err)
	}
	flowInterval, err := time.ParseDuration(env("FLOW_INTERVAL", "2s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse FLOW_INTERVAL: %w", err)
	}
	httpTimeout, err := time.ParseDuration(env("HTTP_TIMEOUT", "10s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse HTTP_TIMEOUT: %w", err)
	}
	return Config{
		BackendURL: env("BACKEND_URL", "http://localhost:8080"), MockMode: mockMode,
		HeartbeatInterval: heartbeat, FlowInterval: flowInterval, HTTPTimeout: httpTimeout,
		BPFObject:        env("BPF_OBJECT", "./ebpf/flow_tracker.bpf.o"),
		CaptureMode:      env("CAPTURE_MODE", "tc"),
		CaptureInterface: env("CAPTURE_INTERFACE", "ens3"),
		AgentID:          os.Getenv("AGENT_ID"),
		Hostname:         os.Getenv("AGENT_HOSTNAME"), MachineID: os.Getenv("MACHINE_ID"),
		TenantID: os.Getenv("TENANT_ID"), PrivateIPs: csv(os.Getenv("AGENT_PRIVATE_IPS")),
		PublicIP: os.Getenv("AGENT_PUBLIC_IP"), MACAddresses: csv(os.Getenv("AGENT_MAC_ADDRESSES")),
		IgnoreIPs:   csv(os.Getenv("IGNORE_IPS")),
		AllowCIDRs:  csv(os.Getenv("FLOW_ALLOW_CIDRS")),
		DenyCIDRs:   csv(os.Getenv("FLOW_DENY_CIDRS")),
		Environment: env("AGENT_ENVIRONMENT", "local"), AgentVersion: env("AGENT_VERSION", "0.1.0"),
	}, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func csv(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}
