package pipeline

import "context"

type Pipeline struct {
	dispatcher *Dispatcher
}

func New(dispatcher *Dispatcher) *Pipeline {
	return &Pipeline{dispatcher: dispatcher}
}

func (p *Pipeline) Handle(ctx context.Context, event Event) error {
	if p == nil || p.dispatcher == nil {
		return nil
	}
	return p.dispatcher.Dispatch(ctx, event)
}
