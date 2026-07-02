package collector

import (
	"context"

	"github.com/vmlens/vmlens/agent/internal/model"
)

type Collector interface {
	Run(context.Context) (<-chan model.FlowEvent, <-chan error)
	Close() error
}
