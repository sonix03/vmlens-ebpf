package packets

import "testing"

func TestApplyAddsPacketCount(t *testing.T) {
	var model Model
	Apply(&model, 2)
	Apply(&model, 3)
	if model.Count != 5 {
		t.Fatalf("count=%d, want 5", model.Count)
	}
}
