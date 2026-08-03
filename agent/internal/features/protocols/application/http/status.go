package http

// StatusCounters aggregates HTTP response status evidence for one flow bucket.
//
// Only the status class counts and the most recent code are kept. The status
// line itself is parsed in the kernel and never reaches userspace, so there is
// no reason phrase, header, or body to retain here.
type StatusCounters struct {
	Informational uint64 // 1xx
	Success       uint64 // 2xx
	Redirect      uint64 // 3xx
	ClientError   uint64 // 4xx
	ServerError   uint64 // 5xx
	Last          int
}

// Total reports how many status lines were observed in this bucket.
func (c StatusCounters) Total() uint64 {
	return c.Informational + c.Success + c.Redirect + c.ClientError + c.ServerError
}

// ApplyStatus records one observed response status. A zero or out-of-range code
// means the packet carried no status line and is ignored.
func ApplyStatus(counters *StatusCounters, status int) {
	if counters == nil || status < 100 || status > 599 {
		return
	}
	switch status / 100 {
	case 1:
		counters.Informational++
	case 2:
		counters.Success++
	case 3:
		counters.Redirect++
	case 4:
		counters.ClientError++
	case 5:
		counters.ServerError++
	default:
		return
	}
	counters.Last = status
}

// MergeStatus folds one bucket's counters into another.
func MergeStatus(current *StatusCounters, next StatusCounters) {
	if current == nil {
		return
	}
	current.Informational += next.Informational
	current.Success += next.Success
	current.Redirect += next.Redirect
	current.ClientError += next.ClientError
	current.ServerError += next.ServerError
	if next.Last != 0 {
		current.Last = next.Last
	}
}
