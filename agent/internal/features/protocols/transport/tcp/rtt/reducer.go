package rtt

func Apply(model *Model, valueMs float64) {
	if model == nil || valueMs <= 0 {
		return
	}
	model.CurrentMs = valueMs
	if model.Samples == 0 || valueMs < model.MinMs {
		model.MinMs = valueMs
	}
	if valueMs > model.MaxMs {
		model.MaxMs = valueMs
	}
	model.Samples++
	model.AvgMs += (valueMs - model.AvgMs) / float64(model.Samples)
}
