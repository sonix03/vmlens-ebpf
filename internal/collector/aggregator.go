package collector

import (
	"context"
	"fmt"
)

type Sink interface {
	Observe(Event) error
}

type Aggregator struct {
	sinks  []Sink
	Errors chan error
	Done   chan struct{}
}

func NewAggregator(sinks ...Sink) *Aggregator {
	return &Aggregator{sinks: sinks, Errors: make(chan error, 16), Done: make(chan struct{})}
}

func (a *Aggregator) Run(ctx context.Context, events <-chan Event) {
	defer close(a.Done)
	defer close(a.Errors)
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			for _, sink := range a.sinks {
				if err := sink.Observe(event); err != nil {
					select {
					case a.Errors <- fmt.Errorf("event sink: %w", err):
					default:
					}
				}
			}
		}
	}
}
