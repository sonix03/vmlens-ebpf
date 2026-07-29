package retrans

import "testing"

func TestApplyAddsRetransCount(t *testing.T) {
	var model Model
	Apply(&model, 2)
	Apply(&model, 4)
	if model.Count != 6 {
		t.Fatalf("count=%d, want 6", model.Count)
	}
}
