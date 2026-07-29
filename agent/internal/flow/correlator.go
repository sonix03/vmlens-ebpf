package flow

import telemetry "github.com/vmlens/vmlens/agent/internal/exporter"

func Correlate(event telemetry.FlowEvent) Key {
	return Key{
		AgentID:   event.AgentID,
		SrcIP:     event.SrcIP,
		DstIP:     event.DstIP,
		DstPort:   event.DstPort,
		Protocol:  event.Protocol,
		Direction: event.Direction,
		Interface: event.Interface,
	}
}
