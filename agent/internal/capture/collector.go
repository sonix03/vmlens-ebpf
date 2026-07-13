package capture

import (
	"context"

	"github.com/vmlens/vmlens/agent/internal/telemetry"
)

type Collector interface {
	Run(context.Context) (<-chan telemetry.FlowEvent, <-chan error)
	Close() error
}
