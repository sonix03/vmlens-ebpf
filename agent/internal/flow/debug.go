package flow

import (
	"fmt"

	telemetry "github.com/vmlens/vmlens/agent/internal/exporter"
)

func DescribeEvent(stage string, event telemetry.FlowEvent) string {
	return fmt.Sprintf(
		"flow_debug stage=%s agent=%s iface=%s %s:%d -> %s:%d protocol=%s direction=%s sent=%d received=%d packets=%d conn=%d req=%d err=%d rtt_ms=%.2f app_ms=%.2f",
		stage,
		event.AgentID,
		event.Interface,
		event.SrcIP,
		event.SrcPort,
		event.DstIP,
		event.DstPort,
		event.Protocol,
		event.Direction,
		event.BytesSent,
		event.BytesReceived,
		event.Packets,
		event.ConnectionCount,
		event.RequestCount,
		event.ErrorCount,
		event.AvgRTTMs,
		event.AvgAppDelayMs,
	)
}

func DescribeBatch(stage string, events []telemetry.FlowEvent) string {
	return fmt.Sprintf("flow_debug stage=%s batch_size=%d", stage, len(events))
}
