package bytes

import "github.com/vmlens/vmlens/agent/internal/features/traffic/direction"

func Apply(model *Model, flowDirection string, byteCount uint64) {
	if model == nil || byteCount == 0 {
		return
	}
	if flowDirection == direction.Ingress {
		model.Received += byteCount
		return
	}
	model.Sent += byteCount
}
