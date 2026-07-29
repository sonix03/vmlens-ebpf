package flow

import telemetry "github.com/vmlens/vmlens/agent/internal/exporter"

func (a *Accumulator) Reset() {
	if a == nil {
		return
	}
	a.flows = make(map[Key]telemetry.FlowEvent)
}
