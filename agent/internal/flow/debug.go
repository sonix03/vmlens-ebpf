package flow

import (
	"fmt"

	"github.com/vmlens/vmlens/agent/internal/pipeline"
)

func DescribeEvent(stage string, event pipeline.FlowMetric) string {
	return fmt.Sprintf(
		"flow_debug stage=%s event=%s source=%s agent=%s iface=%s %s:%d -> %s:%d protocol=%s direction=%s byte_count=%d packets=%d conn=%d req=%d err=%d retrans=%d rtt_ms=%.2f app_ms=%.2f",
		stage,
		event.Type(),
		event.Source,
		event.AgentID,
		event.Interface,
		event.SrcIP,
		event.SrcPort,
		event.DstIP,
		event.DstPort,
		event.Protocol,
		event.Direction,
		event.ByteCount,
		event.PacketCount,
		event.ConnectionCount,
		event.RequestCount,
		event.ErrorCount,
		event.RetransmissionCount,
		event.RTTMs,
		event.AppDelayMs,
	)
}

func DescribeState(stage string, state State) string {
	return fmt.Sprintf(
		"flow_debug stage=%s agent=%s iface=%s %s:%d -> %s:%d protocol=%s direction=%s sent=%d received=%d packets=%d conn=%d req=%d err=%d retrans=%d rtt_avg_ms=%.2f app_avg_ms=%.2f classification=%s/%s/%s",
		stage,
		state.Key.AgentID,
		state.Key.Interface,
		state.Key.SrcIP,
		state.SrcPort,
		state.Key.DstIP,
		state.Key.DstPort,
		state.Key.Protocol,
		state.Key.Direction,
		state.Traffic.Bytes.Sent,
		state.Traffic.Bytes.Received,
		state.Traffic.Packets.Count,
		state.TCP.Connection.OpenCount,
		state.TCP.Connection.RequestHint,
		state.TCP.Connection.ErrorCount,
		state.TCP.Retrans.Count,
		state.TCP.RTT.AvgMs,
		state.Application.AvgDelayMs,
		state.Classification.Network,
		state.Classification.Transport,
		state.Classification.Application,
	)
}

func DescribeBatch(stage string, states []State) string {
	return fmt.Sprintf("flow_debug stage=%s batch_size=%d", stage, len(states))
}
