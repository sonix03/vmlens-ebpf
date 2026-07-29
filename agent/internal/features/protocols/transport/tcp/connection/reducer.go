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
