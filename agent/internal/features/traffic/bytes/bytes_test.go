package bytes

import (
	"testing"

	"github.com/vmlens/vmlens/agent/internal/features/traffic/direction"
)

func TestApplyDirectionalBytesToModel(t *testing.T) {
	var model Model
	Apply(&model, direction.Egress, 12)
	if model.Sent != 12 || model.Received != 0 {
		t.Fatalf("unexpected egress bytes: sent=%d received=%d", model.Sent, model.Received)
	}

	Apply(&model, direction.Ingress, 8)
	if model.Sent != 12 || model.Received != 8 {
		t.Fatalf("unexpected ingress bytes: sent=%d received=%d", model.Sent, model.Received)
	}
}
