package http

func NewTransaction(request Request, response Response, latencyMs float64) Transaction {
	return Transaction{
		Method:     request.Method,
		Path:       request.Path,
		StatusCode: response.StatusCode,
		LatencyMs:  latencyMs,
	}
}
