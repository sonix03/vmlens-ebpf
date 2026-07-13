package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var defaultInternalCIDRs = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"127.0.0.0/8",
}

type Config struct {
	Agent   AgentConfig   `yaml:"agent"`
	VM      VMConfig      `yaml:"vm"`
	Network NetworkConfig `yaml:"network"`
	Privacy PrivacyConfig `yaml:"privacy"`
	FlowLog FlowLogConfig `yaml:"flowlog"`
}

type AgentConfig struct {
	ListenAddr string `yaml:"listen_addr"`
	LogLevel   string `yaml:"log_level"`
}

type VMConfig struct {
	TenantID  string `yaml:"tenant_id"`
	UserID    string `yaml:"user_id"`
	VMID      string `yaml:"vm_id"`
	Hostname  string `yaml:"hostname"`
	PrivateIP string `yaml:"private_ip"`
	PublicIP  string `yaml:"public_ip"`
	Region    string `yaml:"region"`
}

type NetworkConfig struct {
	InternalCIDRs []string `yaml:"internal_cidrs"`
	BPFObject     string   `yaml:"bpf_object"`
}

type PrivacyConfig struct {
	CollectCmdline bool `yaml:"collect_cmdline"`
	RedactSecrets  bool `yaml:"redact_secrets"`
	CollectDNS     bool `yaml:"collect_dns"`
}

type FlowLogConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

func Default() Config {
	return Config{
		Agent: AgentConfig{ListenAddr: ":9109", LogLevel: "info"},
		VM:    VMConfig{VMID: "unknown", Region: "unknown"},
		Network: NetworkConfig{
			InternalCIDRs: append([]string(nil), defaultInternalCIDRs...),
			BPFObject:     "./internal/ebpf/program.bpf.o",
		},
		Privacy: PrivacyConfig{RedactSecrets: true},
		FlowLog: FlowLogConfig{Path: "./data/flows.jsonl"},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config %q: %w", path, err)
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %q: %w", path, err)
	}
	if cfg.Agent.ListenAddr == "" {
		cfg.Agent.ListenAddr = ":9109"
	}
	if cfg.VM.VMID == "" {
		cfg.VM.VMID = "unknown"
	}
	if len(cfg.Network.InternalCIDRs) == 0 {
		cfg.Network.InternalCIDRs = append([]string(nil), defaultInternalCIDRs...)
	}
	if cfg.Network.BPFObject == "" {
		cfg.Network.BPFObject = "./internal/ebpf/program.bpf.o"
	}
	if cfg.FlowLog.Path == "" {
		cfg.FlowLog.Path = "./data/flows.jsonl"
	}
	if cfg.Privacy.CollectCmdline && !cfg.Privacy.RedactSecrets {
		return cfg, fmt.Errorf("privacy.collect_cmdline requires privacy.redact_secrets=true")
	}
	return cfg, nil
}
