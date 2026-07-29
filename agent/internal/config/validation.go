package config

import "fmt"

func Validate(cfg Config) error {
	if cfg.BackendURL == "" {
		return fmt.Errorf("BACKEND_URL is required")
	}
	if !cfg.MockMode && cfg.BPFObject == "" {
		return fmt.Errorf("BPF_OBJECT is required when MOCK_MODE=false")
	}
	if cfg.FlowInterval <= 0 {
		return fmt.Errorf("FLOW_INTERVAL must be positive")
	}
	if cfg.HTTPTimeout <= 0 {
		return fmt.Errorf("HTTP_TIMEOUT must be positive")
	}
	return nil
}
