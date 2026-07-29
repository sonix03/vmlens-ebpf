package connection

import (
	"github.com/vmlens/vmlens/agent/internal/features/classification"
	"github.com/vmlens/vmlens/agent/internal/features/traffic/direction"
)

func InferRequestCount(protocol, flowDirection string, bytes int64, connections uint32, errorCount uint32) int64 {
	if errorCount > 0 {
		return 0
	}
	if connections > 0 {
		return int64(connections)
	}
	if bytes <= 0 {
		return 0
	}
	if protocol != classification.ProtocolUDP && protocol != classification.ProtocolICMP {
		return 0
	}
	if flowDirection == direction.Ingress || flowDirection == direction.Egress {
		return 1
	}
	return 0
}

func Apply(model *Model, event Event) {
	if model == nil {
		return
	}
	if event.Connections > 0 {
		model.OpenCount += uint64(event.Connections)
	}
	if event.Errors > 0 {
		model.ErrorCount += uint64(event.Errors)
	}
	requests := int64(event.Requests)
	if requests == 0 {
		requests = InferRequestCount(event.Protocol, event.Direction, event.Bytes, event.Connections, event.Errors)
	}
	if requests > 0 {
		model.RequestHint += uint64(requests)
	}
}
