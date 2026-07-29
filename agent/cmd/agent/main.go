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

	"github.com/vmlens/vmlens/agent/internal/config"
	"github.com/vmlens/vmlens/agent/internal/exporter"
	"github.com/vmlens/vmlens/agent/internal/features/protocols/network/icmp"
	"github.com/vmlens/vmlens/agent/internal/features/traffic/packet"
	"github.com/vmlens/vmlens/agent/internal/flow"
	"github.com/vmlens/vmlens/agent/internal/identity"
	"github.com/vmlens/vmlens/agent/internal/lifecycle"
	"github.com/vmlens/vmlens/agent/internal/probe"
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
	client := exporter.New(cfg.BackendURL, cfg.HTTPTimeout)
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
	probe.Run(ctx, cfg, client, result.AgentID)

	var source packet.Collector
	if cfg.MockMode {
		source = packet.NewMock(registration, cfg.FlowInterval)
	} else {
		ebpfSource, err := packet.NewEBPF(registration, packet.EBPFOptions{
			ObjectPath:       cfg.BPFObject,
			CaptureMode:      cfg.CaptureMode,
			CaptureInterface: cfg.CaptureInterface,
			IgnorePorts:      cfg.IgnorePorts,
		})
		if err != nil {
			return fmt.Errorf("start real eBPF collector: %w", err)
		}
		source = ebpfSource
		log.Printf("eBPF collector loaded object=%s mode=%s interface=%s", cfg.BPFObject, ebpfSource.Mode(), cfg.CaptureInterface)
		if ebpfSource.Mode() != "tc" {
			icmpSource, err := icmp.NewCollector(registration, cfg.CaptureInterface)
			if err != nil {
				log.Printf("icmp reachability collector disabled: %v", err)
			} else {
				source = packet.NewMulti(ebpfSource, icmpSource)
				log.Printf("icmp reachability collector loaded interface=%s", cfg.CaptureInterface)
			}
		}
	}
	defer source.Close()
	events, collectorErrors := source.Run(ctx)
	flowTicker := time.NewTicker(cfg.FlowInterval)
	defer flowTicker.Stop()
	pendingFlows := flow.NewAccumulator()
	flowBatches := make(chan []exporter.FlowEvent, 32)
	go runFlowSender(ctx, client, flowBatches, cfg.FlowDebug)
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
			if ignoreFlow(controlPlane, event, cfg.IgnorePorts) {
				continue
			}
			if !flowFilter.allows(event) {
				continue
			}
			if cfg.FlowDebug {
				log.Print(flow.DescribeEvent("captured", event))
			}
			pendingFlows.Add(event)
		case <-flowTicker.C:
			batch := pendingFlows.Drain()
			if len(batch) == 0 {
				continue
			}
			if cfg.FlowDebug {
				log.Print(flow.DescribeBatch("drain", batch))
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

func runFlowSender(ctx context.Context, client *exporter.Sender, batches <-chan []exporter.FlowEvent, debug bool) {
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-batches:
			for _, event := range batch {
				if err := sendFlow(ctx, client, event); err != nil {
					log.Printf("send aggregated flow: %v", err)
				} else if debug {
					log.Print(flow.DescribeEvent("exported", event))
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

func ignoreFlow(controlPlane endpointFilter, event exporter.FlowEvent, ignoredPorts []int) bool {
	address := net.ParseIP(event.DstIP)
	if address == nil || address.IsUnspecified() || address.IsLoopback() {
		return true
	}
	if controlPlane.matches(event.DstIP) {
		return true
	}
	for _, port := range ignoredPorts {
		if port == event.SrcPort || port == event.DstPort {
			return true
		}
	}
	return false
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

func (f flowFilter) allows(event exporter.FlowEvent) bool {
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

func registerUntilReady(ctx context.Context, client *exporter.Sender, registration exporter.Registration) (exporter.RegistrationResult, error) {
	delay := time.Second
	for {
		result, err := client.Register(ctx, registration)
		if err == nil {
			return result, nil
		}
		log.Printf("register: %v; retrying in %s", err, delay)
		select {
		case <-ctx.Done():
			return exporter.RegistrationResult{}, ctx.Err()
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

func sendFlow(ctx context.Context, client *exporter.Sender, event exporter.FlowEvent) error {
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
