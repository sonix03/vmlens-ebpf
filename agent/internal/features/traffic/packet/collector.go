package packet

import (
	"context"

	"github.com/vmlens/vmlens/agent/internal/pipeline"
)

type Collector interface {
	Run(context.Context) (<-chan pipeline.FlowMetric, <-chan error)
	Close() error
}
