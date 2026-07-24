package probe

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/vmlens/vmlens/agent/internal/config"
	"github.com/vmlens/vmlens/agent/internal/telemetry"
	"github.com/vmlens/vmlens/agent/internal/transport"
)

const (
	sourceVMLensProbe   = "vmlens_probe"
	typeConnectivity    = "connectivity_check"
	defaultProbeNetwork = "tcp"
)

func Run(ctx context.Context, cfg config.Config, client *transport.Sender, agentID string) {
	if !cfg.ConnectivityProbeEnabled {
		return
	}
	go serveTCP(ctx, cfg.ConnectivityProbeListenAddr)
	go runTargetLoop(ctx, cfg, client, agentID)
}

func serveTCP(ctx context.Context, address string) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Printf("connectivity probe listener disabled: %v", err)
		return
	}
	defer listener.Close()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	log.Printf("connectivity probe listener active address=%s", address)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("connectivity probe listener: %v", err)
			continue
		}
		_ = conn.Close()
	}
}

func runTargetLoop(ctx context.Context, cfg config.Config, client *transport.Sender, agentID string) {
	interval := cfg.ConnectivityProbeInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			probeTargets(ctx, cfg, client, agentID)
		}
	}
}

func probeTargets(ctx context.Context, cfg config.Config, client *transport.Sender, agentID string) {
	targets, err := client.ConnectionTargets(ctx, agentID)
	if err != nil {
		log.Printf("connectivity probe targets: %v", err)
		return
	}
	for _, target := range targets {
		if target.DestIP == "" || target.SourceIP == target.DestIP {
			continue
		}
		event := probeTarget(ctx, cfg, agentID, target)
		if err := client.ConnectionProbe(ctx, event); err != nil {
			log.Printf("connectivity probe result: %v", err)
		}
	}
}

func probeTarget(ctx context.Context, cfg config.Config, agentID string, target telemetry.ConnectionProbeTarget) telemetry.ConnectionProbeEvent {
	protocol := target.Protocol
	if protocol == "" {
		protocol = defaultProbeNetwork
	}
	port := target.DestPort
	if port <= 0 {
		port = 18081
	}
	timeout := cfg.ConnectivityProbeTimeout
	if timeout <= 0 {
		timeout = time.Second
	}
	now := time.Now().UTC()
	start := time.Now()
	success, errText := tcpConnect(ctx, target.DestIP, port, timeout)
	return telemetry.ConnectionProbeEvent{
		AgentID: agentID, SourceIP: target.SourceIP, DestIP: target.DestIP,
		Protocol: protocol, DestPort: port, Success: success,
		RTTMs: float64(time.Since(start).Microseconds()) / 1000,
		Error: errText, Source: sourceVMLensProbe, Type: typeConnectivity,
		CountedAsRequest: false, CountedAsUserTraffic: false, Timestamp: now,
	}
}

func tcpConnect(ctx context.Context, host string, port int, timeout time.Duration) (bool, string) {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	if err != nil {
		return false, err.Error()
	}
	_ = conn.Close()
	return true, ""
}
