package pipeline

import "time"

const FlowMetricEventType = "flow.metric"

type Event interface {
	Type() string
	Time() time.Time
}

// FlowMetric is the internal agent contract between collectors, metric
// reducers, flow state, and the exporter. It intentionally does not use the
// backend payload type. Exporter payloads are created only after flow.State is
// drained.
type FlowMetric struct {
	AgentID string

	SrcIP   string
	DstIP   string
	SrcPort int
	DstPort int

	Protocol  string
	Direction string
	Interface string
	Source    string

	ByteCount           int64
	PacketCount         int64
	ConnectionCount     int64
	RequestCount        int64
	ErrorCount          int64
	RetransmissionCount int64

	RTTMs      float64
	AppDelayMs float64

	// HTTPStatus is the response status derived in-kernel from a plaintext
	// HTTP/1.x status line, or 0 when the packet carried no status line.
	HTTPStatus int

	FirstSeen time.Time
	LastSeen  time.Time
}

func (e FlowMetric) Type() string { return FlowMetricEventType }

func (e FlowMetric) Time() time.Time {
	if !e.LastSeen.IsZero() {
		return e.LastSeen
	}
	if !e.FirstSeen.IsZero() {
		return e.FirstSeen
	}
	return time.Time{}
}
