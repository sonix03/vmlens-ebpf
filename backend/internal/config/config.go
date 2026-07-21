package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	ListenAddr                string
	DatabaseURL               string
	InternalCIDRs             []string
	UnregisteredInternalScope string
	AllowedOrigins            []string
	FlowActiveWindow          time.Duration
	StatusSweepPeriod         time.Duration
	VMDeleteAfter             time.Duration
	DeepFlow                  DeepFlowConfig
}

type DeepFlowConfig struct {
	Enabled                bool
	ClickHouseURL          string
	ClickHouseDatabase     string
	ClickHouseUsername     string
	ClickHousePassword     string
	QuerierURL             string
	ControllerURL          string
	QueryTimeout           time.Duration
	DefaultWindow          time.Duration
	MaxLimit               int
	MaskExternalIPs        bool
	RequireInventoryFilter bool
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:                env("LISTEN_ADDR", ":8080"),
		DatabaseURL:               env("DATABASE_URL", "postgres://vmlens:vmlens@localhost:5432/vmlens?sslmode=disable"),
		InternalCIDRs:             csv(env("INTERNAL_CIDRS", "10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.0/8,fc00::/7,fe80::/10,::1/128")),
		UnregisteredInternalScope: env("UNREGISTERED_INTERNAL_SCOPE", "external_private"),
		AllowedOrigins:            csv(env("ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000")),
		FlowActiveWindow:          3 * time.Second,
		StatusSweepPeriod:         30 * time.Second,
		VMDeleteAfter:             0,
		DeepFlow: DeepFlowConfig{
			Enabled:                envBool("DEEPFLOW_ENABLED", true),
			ClickHouseURL:          env("DEEPFLOW_CLICKHOUSE_URL", "http://host.docker.internal:8123"),
			ClickHouseDatabase:     env("DEEPFLOW_CLICKHOUSE_DATABASE", "default"),
			ClickHouseUsername:     env("DEEPFLOW_CLICKHOUSE_USERNAME", "default"),
			ClickHousePassword:     os.Getenv("DEEPFLOW_CLICKHOUSE_PASSWORD"),
			QuerierURL:             env("DEEPFLOW_QUERIER_URL", "http://host.docker.internal:20416"),
			ControllerURL:          env("DEEPFLOW_CONTROLLER_URL", "http://host.docker.internal:30417"),
			QueryTimeout:           5 * time.Second,
			DefaultWindow:          30 * time.Minute,
			MaxLimit:               1000,
			MaskExternalIPs:        envBool("DEEPFLOW_MASK_EXTERNAL_IPS", false),
			RequireInventoryFilter: envBool("DEEPFLOW_REQUIRE_INVENTORY_FILTER", true),
		},
	}
	if raw := os.Getenv("FLOW_ACTIVE_WINDOW"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse FLOW_ACTIVE_WINDOW: %w", err)
		}
		cfg.FlowActiveWindow = d
	}
	if raw := os.Getenv("STATUS_SWEEP_PERIOD"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse STATUS_SWEEP_PERIOD: %w", err)
		}
		cfg.StatusSweepPeriod = d
	}
	if raw := os.Getenv("VM_DELETE_AFTER"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse VM_DELETE_AFTER: %w", err)
		}
		cfg.VMDeleteAfter = d
	}
	if raw := os.Getenv("DEEPFLOW_QUERY_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse DEEPFLOW_QUERY_TIMEOUT: %w", err)
		}
		cfg.DeepFlow.QueryTimeout = d
	}
	if raw := os.Getenv("DEEPFLOW_DEFAULT_WINDOW"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse DEEPFLOW_DEFAULT_WINDOW: %w", err)
		}
		cfg.DeepFlow.DefaultWindow = d
	}
	if raw := os.Getenv("DEEPFLOW_MAX_LIMIT"); raw != "" {
		var value int
		if _, err := fmt.Sscanf(raw, "%d", &value); err != nil || value < 1 {
			return Config{}, fmt.Errorf("DEEPFLOW_MAX_LIMIT must be a positive integer")
		}
		cfg.DeepFlow.MaxLimit = value
	}
	if cfg.VMDeleteAfter < 0 || (cfg.VMDeleteAfter > 0 && cfg.VMDeleteAfter <= 5*time.Minute) {
		return Config{}, fmt.Errorf("VM_DELETE_AFTER must be 0 (disabled) or greater than 5 minutes")
	}
	if cfg.FlowActiveWindow < 500*time.Millisecond || cfg.FlowActiveWindow > time.Minute {
		return Config{}, fmt.Errorf("FLOW_ACTIVE_WINDOW must be between 500ms and 1m")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.UnregisteredInternalScope != "external_private" && cfg.UnregisteredInternalScope != "unknown_internal" {
		return Config{}, fmt.Errorf("UNREGISTERED_INTERNAL_SCOPE must be external_private or unknown_internal")
	}
	if cfg.DeepFlow.Enabled {
		if cfg.DeepFlow.ClickHouseURL == "" && cfg.DeepFlow.QuerierURL == "" {
			return Config{}, fmt.Errorf("DEEPFLOW_CLICKHOUSE_URL or DEEPFLOW_QUERIER_URL is required when DeepFlow is enabled")
		}
		if cfg.DeepFlow.QueryTimeout < time.Second || cfg.DeepFlow.QueryTimeout > time.Minute {
			return Config{}, fmt.Errorf("DEEPFLOW_QUERY_TIMEOUT must be between 1s and 1m")
		}
		if cfg.DeepFlow.DefaultWindow < time.Minute || cfg.DeepFlow.DefaultWindow > 24*time.Hour {
			return Config{}, fmt.Errorf("DEEPFLOW_DEFAULT_WINDOW must be between 1m and 24h")
		}
		if cfg.DeepFlow.MaxLimit < 1 || cfg.DeepFlow.MaxLimit > 10000 {
			return Config{}, fmt.Errorf("DEEPFLOW_MAX_LIMIT must be between 1 and 10000")
		}
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func csv(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func envBool(key string, fallback bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}
