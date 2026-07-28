package metrics

import "github.com/vmlens/vmlens/backend/internal/model"

func InferRequestCount(event model.FlowEvent) int64 {
	if event.ErrorCount > 0 {
		return 0
	}
	if event.ConnectionCount > 0 {
		return event.ConnectionCount
	}
	if event.Protocol == ProtocolUDP || event.Protocol == ProtocolICMP {
		switch event.Direction {
		case DirectionEgress:
			if event.BytesSent > 0 {
				return 1
			}
		case DirectionIngress:
			if event.BytesReceived > 0 {
				return 1
			}
		}
	}
	return 0
}
