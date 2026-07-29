package http

type Transaction struct {
	Method     string
	Path       string
	StatusCode int
	LatencyMs  float64
}
