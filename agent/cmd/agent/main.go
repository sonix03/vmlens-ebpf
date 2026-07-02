package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vmlens/vmlens/agent/internal/collector"
	"github.com/vmlens/vmlens/agent/internal/config"
	"github.com/vmlens/vmlens/agent/internal/heartbeat"
	"github.com/vmlens/vmlens/agent/internal/identity"
	"github.com/vmlens/vmlens/agent/internal/model"
	"github.com/vmlens/vmlens/agent/internal/sender"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	registration, err := identity.Collect(cfg)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client := sender.New(cfg.BackendURL, cfg.HTTPTimeout)

	result, err := registerUntilReady(ctx, client, registration)
	if err != nil {
		return err
	}
	log.Printf("registered agent=%s vm=%s hostname=%s mock=%t", result.AgentID, result.VMID, registration.Hostname, cfg.MockMode)
	go heartbeat.Run(ctx, registration.AgentID, cfg.HeartbeatInterval, client)

	var source collector.Collector
	if cfg.MockMode {
		source = collector.NewMock(registration, cfg.FlowInterval)
	} else {
		source, err = collector.NewEBPF(registration, cfg.BPFObject)
		if err != nil {
			return fmt.Errorf("start real eBPF collector: %w", err)
		}
		log.Printf("eBPF collector loaded object=%s", cfg.BPFObject)
	}
	defer source.Close()
	events, collectorErrors := source.Run(ctx)
	for events != nil || collectorErrors != nil {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if err := sendFlow(ctx, client, event); err != nil {
				log.Printf("send flow: %v", err)
			}
		case err, ok := <-collectorErrors:
			if !ok {
				collectorErrors = nil
				continue
			}
			if err != nil {
				log.Printf("collector: %v", err)
			}
		}
	}
	return nil
}

func registerUntilReady(ctx context.Context, client *sender.Sender, registration model.Registration) (model.RegistrationResult, error) {
	delay := time.Second
	for {
		result, err := client.Register(ctx, registration)
		if err == nil {
			return result, nil
		}
		log.Printf("register: %v; retrying in %s", err, delay)
		select {
		case <-ctx.Done():
			return model.RegistrationResult{}, ctx.Err()
		case <-time.After(delay):
		}
		if delay < 30*time.Second {
			delay *= 2
		}
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
	}
}

func sendFlow(ctx context.Context, client *sender.Sender, event model.FlowEvent) error {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if err := client.Flow(ctx, event); err == nil {
			return nil
		} else {
			last = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
		}
	}
	return last
}
