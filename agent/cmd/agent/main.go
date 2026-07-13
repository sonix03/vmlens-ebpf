package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vmlens/vmlens/agent/internal/capture"
	"github.com/vmlens/vmlens/agent/internal/config"
	"github.com/vmlens/vmlens/agent/internal/identity"
	"github.com/vmlens/vmlens/agent/internal/lifecycle"
	"github.com/vmlens/vmlens/agent/internal/telemetry"
	"github.com/vmlens/vmlens/agent/internal/transport"
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
	client := transport.New(cfg.BackendURL, cfg.HTTPTimeout)
	controlPlane := newEndpointFilter(cfg.BackendURL, cfg.IgnoreIPs)
	flowFilter, err := newFlowFilter(cfg.AllowCIDRs, cfg.DenyCIDRs)
	if err != nil {
		return err
	}

	result, err := registerUntilReady(ctx, client, registration)
	if err != nil {
		return err
	}
	log.Printf("registered agent=%s vm=%s hostname=%s mock=%t", result.AgentID, result.VMID, registration.Hostname, cfg.MockMode)
	go lifecycle.Run(ctx, registration, cfg.HeartbeatInterval, client)

	var source capture.Collector
	if cfg.MockMode {
		source = capture.NewMock(registration, cfg.FlowInterval)
	} else {
		source, err = capture.NewEBPF(registration, capture.EBPFOptions{
			ObjectPath:       cfg.BPFObject,
			CaptureMode:      cfg.CaptureMode,
			CaptureInterface: cfg.CaptureInterface,
		})
		if err != nil {
			return fmt.Errorf("start real eBPF collector: %w", err)
		}
		log.Printf("eBPF collector loaded object=%s mode=%s interface=%s", cfg.BPFObject, cfg.CaptureMode, cfg.CaptureInterface)
	}
	defer source.Close()
	events, collectorErrors := source.Run(ctx)
	flowTicker := time.NewTicker(cfg.FlowInterval)
	defer flowTicker.Stop()
	pendingFlows := newFlowAccumulator()
	flowBatches := make(chan []telemetry.FlowEvent, 32)
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
			if !flowFilter.allows(event) {
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
	flows map[flowKey]telemetry.FlowEvent
}

func newFlowAccumulator() *flowAccumulator {
	return &flowAccumulator{flows: make(map[flowKey]telemetry.FlowEvent)}
}

func (a *flowAccumulator) Add(event telemetry.FlowEvent) {
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
	current.RequestCount += event.RequestCount
	if current.FirstSeen.IsZero() || (!event.FirstSeen.IsZero() && event.FirstSeen.Before(current.FirstSeen)) {
		current.FirstSeen = event.FirstSeen
	}
	if event.LastSeen.After(current.LastSeen) {
		current.LastSeen = event.LastSeen
	}
	a.flows[key] = current
}

func (a *flowAccumulator) AddAll(events []telemetry.FlowEvent) {
	for _, event := range events {
		a.Add(event)
	}
}

func (a *flowAccumulator) Drain() []telemetry.FlowEvent {
	if len(a.flows) == 0 {
		return nil
	}
	events := make([]telemetry.FlowEvent, 0, len(a.flows))
	for _, event := range a.flows {
		events = append(events, event)
	}
	a.flows = make(map[flowKey]telemetry.FlowEvent)
	return events
}

func runFlowSender(ctx context.Context, client *transport.Sender, batches <-chan []telemetry.FlowEvent) {
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

type flowFilter struct {
	allow []netip.Prefix
	deny  []netip.Prefix
}

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

func newFlowFilter(rawAllow, rawDeny []string) (flowFilter, error) {
	parse := func(values []string, label string) ([]netip.Prefix, error) {
		prefixes := make([]netip.Prefix, 0, len(values))
		for _, value := range values {
			prefix, err := netip.ParsePrefix(value)
			if err != nil {
				addr, addrErr := netip.ParseAddr(value)
				if addrErr != nil {
					return nil, fmt.Errorf("parse %s %q: %w", label, value, err)
				}
				prefix = netip.PrefixFrom(addr, addr.BitLen())
			}
			prefixes = append(prefixes, prefix.Masked())
		}
		return prefixes, nil
	}
	allow, err := parse(rawAllow, "FLOW_ALLOW_CIDRS")
	if err != nil {
		return flowFilter{}, err
	}
	deny, err := parse(rawDeny, "FLOW_DENY_CIDRS")
	if err != nil {
		return flowFilter{}, err
	}
	return flowFilter{allow: allow, deny: deny}, nil
}

func (f flowFilter) allows(event telemetry.FlowEvent) bool {
	src, srcOK := parseAddr(event.SrcIP)
	dst, dstOK := parseAddr(event.DstIP)
	if !srcOK && !dstOK {
		return false
	}
	if f.matches(f.deny, src, srcOK) || f.matches(f.deny, dst, dstOK) {
		return false
	}
	if len(f.allow) == 0 {
		return true
	}
	return f.matches(f.allow, src, srcOK) || f.matches(f.allow, dst, dstOK)
}

func (f flowFilter) matches(prefixes []netip.Prefix, addr netip.Addr, ok bool) bool {
	if !ok {
		return false
	}
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func parseAddr(value string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr, true
}

func registerUntilReady(ctx context.Context, client *transport.Sender, registration telemetry.Registration) (telemetry.RegistrationResult, error) {
	delay := time.Second
	for {
		result, err := client.Register(ctx, registration)
		if err == nil {
			return result, nil
		}
		log.Printf("register: %v; retrying in %s", err, delay)
		select {
		case <-ctx.Done():
			return telemetry.RegistrationResult{}, ctx.Err()
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

func sendFlow(ctx context.Context, client *transport.Sender, event telemetry.FlowEvent) error {
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
