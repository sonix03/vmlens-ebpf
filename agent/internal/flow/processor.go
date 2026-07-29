package flow

import "github.com/vmlens/vmlens/agent/internal/pipeline"

type Processor struct {
	store *Accumulator
}

func NewProcessor() *Processor {
	return &Processor{store: NewAccumulator()}
}

func (p *Processor) Handle(event pipeline.FlowMetric) {
	if p == nil || p.store == nil {
		return
	}
	p.store.Add(event)
}

func (p *Processor) Drain() []State {
	if p == nil || p.store == nil {
		return nil
	}
	return p.store.Drain()
}
