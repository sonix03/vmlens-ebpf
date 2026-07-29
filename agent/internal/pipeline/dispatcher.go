package pipeline

import "context"

type Handler func(context.Context, Event) error

type Dispatcher struct {
	handlers map[string]Handler
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{handlers: map[string]Handler{}}
}

func (d *Dispatcher) Register(eventType string, handler Handler) {
	d.handlers[eventType] = handler
}

func (d *Dispatcher) Dispatch(ctx context.Context, event Event) error {
	handler, ok := d.handlers[event.Type()]
	if !ok {
		return nil
	}
	return handler(ctx, event)
}
