package dns

type Query struct {
	Name       string
	Type       uint16
	ResponseMs float64
}
