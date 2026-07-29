package flow

import telemetry "github.com/vmlens/vmlens/agent/internal/exporter"

type Accumulator struct {
	flows map[Key]telemetry.FlowEvent
}

func NewAccumulator() *Accumulator {
	return &Accumulator{flows: make(map[Key]telemetry.FlowEvent)}
}

func (a *Accumulator) Add(event telemetry.FlowEvent) {
	key := Key{
		AgentID:   event.AgentID,
		SrcIP:     event.SrcIP,
		DstIP:     event.DstIP,
		DstPort:   event.DstPort,
		Protocol:  event.Protocol,
		Direction: event.Direction,
		Interface: event.Interface,
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
	current.ErrorCount += event.ErrorCount
	if current.FirstSeen.IsZero() || (!event.FirstSeen.IsZero() && event.FirstSeen.Before(current.FirstSeen)) {
		current.FirstSeen = event.FirstSeen
	}
	if event.LastSeen.After(current.LastSeen) {
		current.LastSeen = event.LastSeen
	}
	a.flows[key] = current
}

func (a *Accumulator) AddAll(events []telemetry.FlowEvent) {
	for _, event := range events {
		a.Add(event)
	}
}

func (a *Accumulator) Drain() []telemetry.FlowEvent {
	if len(a.flows) == 0 {
		return nil
	}
	events := make([]telemetry.FlowEvent, 0, len(a.flows))
	for _, event := range a.flows {
		events = append(events, event)
	}
	a.flows = make(map[Key]telemetry.FlowEvent)
	return events
}
