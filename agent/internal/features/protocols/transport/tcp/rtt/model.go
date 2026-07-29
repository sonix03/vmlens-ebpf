package rtt

type Model struct {
	CurrentMs float64
	MinMs     float64
	MaxMs     float64
	AvgMs     float64
	Samples   uint64
}
