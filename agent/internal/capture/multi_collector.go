package capture

import (
	"context"
	"sync"

	"github.com/vmlens/vmlens/agent/internal/telemetry"
)

type MultiCollector struct {
	collectors []Collector
}

func NewMulti(collectors ...Collector) *MultiCollector {
	return &MultiCollector{collectors: collectors}
}

func (c *MultiCollector) Run(ctx context.Context) (<-chan telemetry.FlowEvent, <-chan error) {
	events := make(chan telemetry.FlowEvent, 1024)
	errorsChannel := make(chan error, len(c.collectors))
	var wg sync.WaitGroup

	for _, collector := range c.collectors {
		source := collector
		sourceEvents, sourceErrors := source.Run(ctx)
		wg.Add(2)
		go func() {
			defer wg.Done()
			for event := range sourceEvents {
				select {
				case events <- event:
				case <-ctx.Done():
					return
				}
			}
		}()
		go func() {
			defer wg.Done()
			for err := range sourceErrors {
				select {
				case errorsChannel <- err:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(events)
		close(errorsChannel)
	}()
	return events, errorsChannel
}

func (c *MultiCollector) Close() error {
	var first error
	for _, collector := range c.collectors {
		if err := collector.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
