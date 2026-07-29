package http

type Correlator struct{}

func (Correlator) Correlate(request Request, response Response, latencyMs float64) Transaction {
	return NewTransaction(request, response, latencyMs)
}
