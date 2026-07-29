package packet

import (
	"context"

	telemetry "github.com/vmlens/vmlens/agent/internal/exporter"
)

type Collector interface {
	Run(context.Context) (<-chan telemetry.FlowEvent, <-chan error)
	Close() error
}
