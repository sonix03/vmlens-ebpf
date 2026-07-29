package exporter

type Batcher struct {
	events []FlowEvent
	limit  int
}

func NewBatcher(limit int) *Batcher {
	if limit <= 0 {
		limit = 100
	}
	return &Batcher{limit: limit}
}

func (b *Batcher) Add(event FlowEvent) []FlowEvent {
	if b == nil {
		return nil
	}
	b.events = append(b.events, event)
	if len(b.events) < b.limit {
		return nil
	}
	return b.Flush()
}

func (b *Batcher) Flush() []FlowEvent {
	if b == nil || len(b.events) == 0 {
		return nil
	}
	out := append([]FlowEvent(nil), b.events...)
	b.events = nil
	return out
}
