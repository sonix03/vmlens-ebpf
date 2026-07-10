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
