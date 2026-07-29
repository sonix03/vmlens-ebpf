package rtt

import "testing"

func TestApplyTracksAverage(t *testing.T) {
	var model Model
	Apply(&model, 10)
	Apply(&model, 20)
	if model.CurrentMs != 20 || model.MinMs != 10 || model.MaxMs != 20 || model.AvgMs != 15 {
		t.Fatalf("unexpected RTT model: %+v", model)
	}
}
