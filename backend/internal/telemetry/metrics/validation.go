package metrics

import (
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/vmlens/vmlens/backend/internal/model"
)

func ValidateFlowEvent(event *model.FlowEvent) error {
	event.AgentID = strings.TrimSpace(event.AgentID)
	event.Protocol = strings.ToLower(strings.TrimSpace(event.Protocol))
	event.Direction = strings.ToLower(strings.TrimSpace(event.Direction))
	if event.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if event.Protocol != ProtocolTCP && event.Protocol != ProtocolUDP && event.Protocol != ProtocolICMP {
		return fmt.Errorf("protocol must be tcp, udp, or icmp")
	}
	event.SrcPort, event.DstPort = NormalizePorts(event.Protocol, event.SrcPort, event.DstPort)
	if event.Direction == "" {
		event.Direction = DirectionEgress
	}
	if event.Direction != DirectionIngress && event.Direction != DirectionEgress {
		return fmt.Errorf("direction must be ingress or egress")
	}
	if _, err := netip.ParseAddr(event.SrcIP); err != nil {
		return fmt.Errorf("invalid src_ip: %w", err)
	}
	if _, err := netip.ParseAddr(event.DstIP); err != nil {
		return fmt.Errorf("invalid dst_ip: %w", err)
	}
	if err := ValidatePortRange(event.SrcPort, event.DstPort); err != nil {
		return err
	}
	if event.BytesSent < 0 || event.BytesReceived < 0 || event.Packets < 0 || event.ConnectionCount < 0 || event.RequestCount < 0 || event.ErrorCount < 0 {
		return fmt.Errorf("flow counters cannot be negative")
	}
	now := time.Now().UTC()
	if event.FirstSeen.IsZero() {
		event.FirstSeen = now
	}
	if event.LastSeen.IsZero() {
		event.LastSeen = event.FirstSeen
	}
	if event.LastSeen.Before(event.FirstSeen) {
		return fmt.Errorf("last_seen cannot be before first_seen")
	}
	if event.RequestCount == 0 {
		event.RequestCount = InferRequestCount(*event)
	}
	return nil
}
