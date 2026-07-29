package retrans

import "context"

type Collector struct{}

func (Collector) Run(context.Context) (<-chan Event, <-chan error) {
	events := make(chan Event)
	errs := make(chan error)
	close(events)
	close(errs)
	return events, errs
}
