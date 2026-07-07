package collector

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"github.com/vmlens/vmlens/agent/internal/model"
)

type mockDestination struct {
	ip       string
	port     int
	protocol string
}

type MockCollector struct {
	registration model.Registration
	interval     time.Duration
	random       *rand.Rand
}

func NewMock(registration model.Registration, interval time.Duration) *MockCollector {
	return &MockCollector{registration: registration, interval: interval, random: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (c *MockCollector) Run(ctx context.Context) (<-chan model.FlowEvent, <-chan error) {
	events := make(chan model.FlowEvent, 64)
	errors := make(chan error)
	go func() {
		defer close(events)
		defer close(errors)
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				events <- c.event(now.UTC())
			}
		}
	}()
	return events, errors
}

func (c *MockCollector) Close() error { return nil }

func (c *MockCollector) event(now time.Time) model.FlowEvent {
	destinations := c.destinations()
	destination := destinations[c.random.Intn(len(destinations))]
	bytesReceived := int64(25_000 + c.random.Intn(2_500_000))
	bytesSent := int64(2_000 + c.random.Intn(350_000))
	connections := int64(1 + c.random.Intn(3))
	sourceIP := "10.10.1.15"
	if len(c.registration.PrivateIPs) > 0 {
		sourceIP = c.registration.PrivateIPs[0]
	}
	iface := "eth0"
	if len(c.registration.Interfaces) > 0 {
		iface = c.registration.Interfaces[0].Name
	}
	return model.FlowEvent{
		AgentID: c.registration.AgentID, SrcIP: sourceIP, DstIP: destination.ip,
		SrcPort: 32000 + c.random.Intn(25000), DstPort: destination.port,
		Protocol: destination.protocol, Direction: "egress", BytesSent: bytesSent,
		BytesReceived: bytesReceived, Packets: (bytesSent+bytesReceived)/1200 + 1,
		ConnectionCount: connections, RequestCount: connections, FirstSeen: now.Add(-time.Duration(100+c.random.Intn(900)) * time.Millisecond),
		LastSeen: now, Interface: iface,
	}
}

func (c *MockCollector) destinations() []mockDestination {
	name := strings.ToLower(c.registration.Hostname)
	if strings.Contains(name, "db") {
		return []mockDestination{{"10.10.1.15", 8080, "tcp"}, {"1.1.1.1", 443, "tcp"}, {"10.10.1.40", 6379, "tcp"}}
	}
	return []mockDestination{{"10.10.1.30", 5432, "tcp"}, {"8.8.8.8", 443, "tcp"}, {"10.10.1.99", 8080, "tcp"}, {"1.1.1.1", 53, "udp"}}
}
