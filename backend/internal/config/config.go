package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	ListenAddr        string
	DatabaseURL       string
	InternalCIDRs     []string
	AllowedOrigins    []string
	StatusSweepPeriod time.Duration
	VMDeleteAfter     time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:        env("LISTEN_ADDR", ":8080"),
		DatabaseURL:       env("DATABASE_URL", "postgres://vmlens:vmlens@localhost:5432/vmlens?sslmode=disable"),
		InternalCIDRs:     csv(env("INTERNAL_CIDRS", "10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.0/8")),
		AllowedOrigins:    csv(env("ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000")),
		StatusSweepPeriod: 30 * time.Second,
		VMDeleteAfter:     15 * time.Minute,
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
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
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
