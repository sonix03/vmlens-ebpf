package flow

import "github.com/vmlens/vmlens/agent/internal/pipeline"

func Correlate(event pipeline.FlowMetric) Key {
	return KeyFromMetric(event)
}
