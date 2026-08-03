package flow

import (
	"github.com/vmlens/vmlens/agent/internal/features/classification"
	"github.com/vmlens/vmlens/agent/internal/features/protocols/transport/tcp/connection"
	"github.com/vmlens/vmlens/agent/internal/features/protocols/transport/tcp/retrans"
	"github.com/vmlens/vmlens/agent/internal/features/protocols/transport/tcp/rtt"
	flowbytes "github.com/vmlens/vmlens/agent/internal/features/traffic/bytes"
	"github.com/vmlens/vmlens/agent/internal/features/traffic/packets"
	"github.com/vmlens/vmlens/agent/internal/pipeline"
)

type Accumulator struct {
	flows map[Key]*State
}

func NewAccumulator() *Accumulator {
	return &Accumulator{flows: make(map[Key]*State)}
}

func (a *Accumulator) Add(event pipeline.FlowMetric) {
	key := KeyFromMetric(event)
	current, exists := a.flows[key]
	if !exists {
		current = &State{Key: key}
		a.flows[key] = current
	}
	current.Apply(event)
}

func (a *Accumulator) AddState(state State) {
	key := state.Key
	current, exists := a.flows[key]
	if !exists {
		copy := state
		a.flows[key] = &copy
		return
	}
	current.Merge(state)
}

func (a *Accumulator) AddAll(states []State) {
	for _, state := range states {
		a.AddState(state)
	}
}

func (a *Accumulator) Drain() []State {
	if len(a.flows) == 0 {
		return nil
	}
	states := make([]State, 0, len(a.flows))
	for _, state := range a.flows {
		states = append(states, *state)
	}
	a.flows = make(map[Key]*State)
	return states
}

func (s *State) Apply(event pipeline.FlowMetric) {
	if s == nil {
		return
	}
	if s.SrcPort == 0 && event.SrcPort > 0 {
		s.SrcPort = event.SrcPort
	}
	flowbytes.Apply(&s.Traffic.Bytes, event.Direction, uint64(nonNegative(event.ByteCount)))
	packets.Apply(&s.Traffic.Packets, uint64(nonNegative(event.PacketCount)))
	s.Traffic.Direction.Current = event.Direction
	connection.Apply(&s.TCP.Connection, connection.Event{
		Protocol: event.Protocol, Direction: event.Direction, Bytes: event.ByteCount,
		Connections: uint32(nonNegative(event.ConnectionCount)), Requests: uint32(nonNegative(event.RequestCount)),
		Errors: uint32(nonNegative(event.ErrorCount)),
	})
	rtt.Apply(&s.TCP.RTT, event.RTTMs)
	retrans.Apply(&s.TCP.Retrans, uint64(nonNegative(event.RetransmissionCount)))
	applyAppDelay(&s.Application, event.AppDelayMs)
	s.Classification = classification.Model{
		Network:     networkFromIP(event.SrcIP),
		Transport:   event.Protocol,
		Application: classification.ApplicationFromPort(event.Protocol, event.DstPort),
	}
	if s.FirstSeen.IsZero() || (!event.FirstSeen.IsZero() && event.FirstSeen.Before(s.FirstSeen)) {
		s.FirstSeen = event.FirstSeen
	}
	if event.LastSeen.After(s.LastSeen) {
		s.LastSeen = event.LastSeen
	}
}

func (s *State) Merge(next State) {
	if s == nil {
		return
	}
	if s.SrcPort == 0 && next.SrcPort > 0 {
		s.SrcPort = next.SrcPort
	}
	s.Traffic.Bytes.Sent += next.Traffic.Bytes.Sent
	s.Traffic.Bytes.Received += next.Traffic.Bytes.Received
	s.Traffic.Packets.Count += next.Traffic.Packets.Count
	s.Traffic.Direction.Current = firstNonEmpty(next.Traffic.Direction.Current, s.Traffic.Direction.Current)
	s.TCP.Connection.OpenCount += next.TCP.Connection.OpenCount
	s.TCP.Connection.ErrorCount += next.TCP.Connection.ErrorCount
	s.TCP.Connection.RequestHint += next.TCP.Connection.RequestHint
	mergeRTT(&s.TCP.RTT, next.TCP.RTT)
	s.TCP.Retrans.Count += next.TCP.Retrans.Count
	mergeAppDelay(&s.Application, next.Application)
	if next.Classification != (classification.Model{}) {
		s.Classification = next.Classification
	}
	if s.FirstSeen.IsZero() || (!next.FirstSeen.IsZero() && next.FirstSeen.Before(s.FirstSeen)) {
		s.FirstSeen = next.FirstSeen
	}
	if next.LastSeen.After(s.LastSeen) {
		s.LastSeen = next.LastSeen
	}
}

func applyAppDelay(model *ApplicationState, valueMs float64) {
	if model == nil || valueMs <= 0 {
		return
	}
	model.Samples++
	model.AvgDelayMs += (valueMs - model.AvgDelayMs) / float64(model.Samples)
}

func mergeAppDelay(current *ApplicationState, next ApplicationState) {
	if current == nil || next.Samples == 0 {
		return
	}
	total := current.Samples + next.Samples
	if total == 0 {
		return
	}
	current.AvgDelayMs = ((current.AvgDelayMs * float64(current.Samples)) + (next.AvgDelayMs * float64(next.Samples))) / float64(total)
	current.Samples = total
}

func mergeRTT(current *rtt.Model, next rtt.Model) {
	if current == nil || next.Samples == 0 {
		return
	}
	if current.Samples == 0 {
		*current = next
		return
	}
	total := current.Samples + next.Samples
	current.AvgMs = ((current.AvgMs * float64(current.Samples)) + (next.AvgMs * float64(next.Samples))) / float64(total)
	current.Samples = total
	current.CurrentMs = next.CurrentMs
	if current.MinMs == 0 || (next.MinMs > 0 && next.MinMs < current.MinMs) {
		current.MinMs = next.MinMs
	}
	if next.MaxMs > current.MaxMs {
		current.MaxMs = next.MaxMs
	}
}

func nonNegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func networkFromIP(ip string) string {
	for _, char := range ip {
		if char == ':' {
			return classification.NetworkIPv6
		}
	}
	if ip != "" {
		return classification.NetworkIPv4
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
