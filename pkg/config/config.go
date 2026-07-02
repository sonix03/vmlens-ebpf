package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     Server     `yaml:"server"`
	Logging    Logging    `yaml:"logging"`
	SSH        SSH        `yaml:"ssh"`
	Privacy    Privacy    `yaml:"privacy"`
	Metrics    Metrics    `yaml:"metrics"`
	Analysis   Analysis   `yaml:"analysis"`
	Collection Collection `yaml:"collection"`
}
type Server struct {
	ListenAddr string `yaml:"listen_addr"`
}
type Logging struct {
	Dir             string `yaml:"dir"`
	SSHLogPath      string `yaml:"ssh_log_path"`
	ProcessLogPath  string `yaml:"process_log_path"`
	ResourceLogPath string `yaml:"resource_log_path"`
	NetworkLogPath  string `yaml:"network_log_path"`
	AnalysisLogPath string `yaml:"analysis_log_path"`
	RetentionDays   int    `yaml:"retention_days"`
}
type SSH struct {
	Enabled       bool   `yaml:"enabled"`
	AuthLogPath   string `yaml:"auth_log_path"`
	ParseJournald bool   `yaml:"parse_journald"`
	ParseAuthLog  bool   `yaml:"parse_auth_log"`
}
type Privacy struct {
	CapturePayload        bool `yaml:"capture_payload"`
	CaptureKeystrokes     bool `yaml:"capture_keystrokes"`
	CaptureTerminalOutput bool `yaml:"capture_terminal_output"`
	LogRemoteIP           bool `yaml:"log_remote_ip"`
	LogDestinationIP      bool `yaml:"log_destination_ip"`
	AnonymizeIP           bool `yaml:"anonymize_ip"`
	SanitizeCommandArgs   bool `yaml:"sanitize_command_args"`
	EnableRemoteUpload    bool `yaml:"enable_remote_upload"`
}
type Metrics struct {
	EnableProcessLabels bool `yaml:"enable_process_labels"`
	EnableIPLabels      bool `yaml:"enable_ip_labels"`
	EnableSessionLabels bool `yaml:"enable_session_labels"`
	EnableCommandLabels bool `yaml:"enable_command_labels"`
}
type Analysis struct {
	CPUHighPercent            float64 `yaml:"cpu_high_percent"`
	CPUHighDurationSeconds    int     `yaml:"cpu_high_duration_seconds"`
	MemoryHighBytes           uint64  `yaml:"memory_high_bytes"`
	NetworkHighBytesPerMinute uint64  `yaml:"network_high_bytes_per_minute"`
	DiskHighBytesPerMinute    uint64  `yaml:"disk_high_bytes_per_minute"`
}
type Collection struct {
	SampleInterval     time.Duration `yaml:"-"`
	SampleIntervalText string        `yaml:"sample_interval"`
	BPFDir             string        `yaml:"bpf_dir"`
}

func Default() Config {
	var c Config
	c.Server.ListenAddr = "127.0.0.1:9435"
	c.Logging.Dir = "/var/log/vmlens"
	c.Logging.SSHLogPath = "/var/log/vmlens/ssh_sessions.log"
	c.Logging.ProcessLogPath = "/var/log/vmlens/process_events.log"
	c.Logging.ResourceLogPath = "/var/log/vmlens/resource_events.log"
	c.Logging.NetworkLogPath = "/var/log/vmlens/network_flows.log"
	c.Logging.AnalysisLogPath = "/var/log/vmlens/analysis.log"
	c.Logging.RetentionDays = 7
	c.SSH.Enabled, c.SSH.ParseJournald, c.SSH.ParseAuthLog = true, true, true
	c.SSH.AuthLogPath = "/var/log/auth.log"
	c.Privacy.LogRemoteIP, c.Privacy.LogDestinationIP, c.Privacy.SanitizeCommandArgs = true, true, true
	c.Metrics.EnableProcessLabels = true
	c.Analysis.CPUHighPercent, c.Analysis.CPUHighDurationSeconds = 80, 30
	c.Analysis.MemoryHighBytes = 1 << 30
	c.Analysis.NetworkHighBytesPerMinute = 100 << 20
	c.Analysis.DiskHighBytesPerMinute = 1 << 30
	c.Collection.SampleInterval, c.Collection.SampleIntervalText = 5*time.Second, "5s"
	c.Collection.BPFDir = "/usr/lib/vmlens/bpf"
	return c
}

func Load(path string) (Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("parse config: %w", err)
	}
	if c.Collection.SampleIntervalText != "" {
		c.Collection.SampleInterval, err = time.ParseDuration(c.Collection.SampleIntervalText)
		if err != nil {
			return c, fmt.Errorf("invalid collection.sample_interval: %w", err)
		}
	}
	if c.Privacy.CapturePayload || c.Privacy.CaptureKeystrokes || c.Privacy.CaptureTerminalOutput || c.Privacy.EnableRemoteUpload {
		return c, fmt.Errorf("unsafe privacy options are not supported")
	}
	return c, nil
}
