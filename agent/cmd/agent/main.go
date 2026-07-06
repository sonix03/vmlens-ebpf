package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
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
	controlPlane := newEndpointFilter(cfg.BackendURL, cfg.IgnoreIPs)

	result, err := registerUntilReady(ctx, client, registration)
	if err != nil {
		return err
	}
	log.Printf("registered agent=%s vm=%s hostname=%s mock=%t", result.AgentID, result.VMID, registration.Hostname, cfg.MockMode)
	go heartbeat.Run(ctx, registration, cfg.HeartbeatInterval, client)

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
	flowTicker := time.NewTicker(cfg.FlowInterval)
	defer flowTicker.Stop()
	pendingFlows := newFlowAccumulator()
	flowBatches := make(chan []model.FlowEvent, 32)
	go runFlowSender(ctx, client, flowBatches)
	for events != nil || collectorErrors != nil {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			// Do not observe the observability transport itself. Sending an ingest
			// request creates socket activity; forwarding that activity again would
			// create a self-amplifying control-plane feedback loop.
			if ignoreFlow(controlPlane, event.DstIP) {
				continue
			}
			pendingFlows.Add(event)
		case <-flowTicker.C:
			batch := pendingFlows.Drain()
			if len(batch) == 0 {
				continue
			}
			select {
			case flowBatches <- batch:
			default:
				// Preserve counters during a temporary backend/tunnel slowdown. The
				// next interval will retry the merged flow instead of dropping bytes.
				pendingFlows.AddAll(batch)
				log.Printf("flow sender backlog full; retaining %d aggregated flows", len(batch))
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

type flowKey struct {
	agentID   string
	srcIP     string
	dstIP     string
	dstPort   int
	protocol  string
	direction string
	iface     string
}

type flowAccumulator struct {
	flows map[flowKey]model.FlowEvent
}

func newFlowAccumulator() *flowAccumulator {
	return &flowAccumulator{flows: make(map[flowKey]model.FlowEvent)}
}

func (a *flowAccumulator) Add(event model.FlowEvent) {
	key := flowKey{
		agentID: event.AgentID, srcIP: event.SrcIP, dstIP: event.DstIP,
		dstPort: event.DstPort, protocol: event.Protocol, direction: event.Direction,
		iface: event.Interface,
	}
	current, exists := a.flows[key]
	if !exists {
		a.flows[key] = event
		return
	}
	current.BytesSent += event.BytesSent
	current.BytesReceived += event.BytesReceived
	current.Packets += event.Packets
	current.ConnectionCount += event.ConnectionCount
	if current.FirstSeen.IsZero() || (!event.FirstSeen.IsZero() && event.FirstSeen.Before(current.FirstSeen)) {
		current.FirstSeen = event.FirstSeen
	}
	if event.LastSeen.After(current.LastSeen) {
		current.LastSeen = event.LastSeen
	}
	a.flows[key] = current
}

func (a *flowAccumulator) AddAll(events []model.FlowEvent) {
	for _, event := range events {
		a.Add(event)
	}
}

func (a *flowAccumulator) Drain() []model.FlowEvent {
	if len(a.flows) == 0 {
		return nil
	}
	events := make([]model.FlowEvent, 0, len(a.flows))
	for _, event := range a.flows {
		events = append(events, event)
	}
	a.flows = make(map[flowKey]model.FlowEvent)
	return events
}

func runFlowSender(ctx context.Context, client *sender.Sender, batches <-chan []model.FlowEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-batches:
			for _, event := range batch {
				if err := sendFlow(ctx, client, event); err != nil {
					log.Printf("send aggregated flow: %v", err)
				}
			}
		}
	}
}

type endpointFilter struct{ addresses map[string]struct{} }

func newEndpointFilter(rawURL string, ignoredIPs []string) endpointFilter {
	filter := endpointFilter{addresses: map[string]struct{}{}}
	for _, raw := range ignoredIPs {
		if ip := net.ParseIP(raw); ip != nil {
			filter.addresses[ip.String()] = struct{}{}
		}
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return filter
	}
	host := parsed.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		filter.addresses[ip.String()] = struct{}{}
		return filter
	}
	if addresses, err := net.LookupIP(host); err == nil {
		for _, address := range addresses {
			filter.addresses[address.String()] = struct{}{}
		}
	}
	return filter
}

func (f endpointFilter) matches(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	_, excluded := f.addresses[parsed.String()]
	return excluded
}

func ignoreFlow(controlPlane endpointFilter, destination string) bool {
	address := net.ParseIP(destination)
	if address == nil || address.IsUnspecified() || address.IsLoopback() {
		return true
	}
	return controlPlane.matches(destination)
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
