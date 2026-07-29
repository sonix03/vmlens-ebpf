package flow

import "github.com/vmlens/vmlens/agent/internal/pipeline"

type Key struct {
	AgentID   string
	SrcIP     string
	DstIP     string
	DstPort   int
	Protocol  string
	Direction string
	Interface string
}

func KeyFromMetric(event pipeline.FlowMetric) Key {
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
