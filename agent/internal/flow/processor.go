package flow

import telemetry "github.com/vmlens/vmlens/agent/internal/exporter"

type Processor struct {
	store *Accumulator
}

func NewProcessor() *Processor {
	return &Processor{store: NewAccumulator()}
}

func (p *Processor) Handle(event telemetry.FlowEvent) {
	if p == nil || p.store == nil {
		return
	}
	p.store.Add(event)
}

func (p *Processor) Drain() []telemetry.FlowEvent {
	if p == nil || p.store == nil {
		return nil
	}
	return p.store.Drain()
}
