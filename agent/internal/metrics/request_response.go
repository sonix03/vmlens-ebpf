package metrics

func InferRequestCount(protocol, direction string, bytes int64, connections uint32, errorCount uint32) int64 {
	if errorCount > 0 {
		return 0
	}
	if connections > 0 {
		return int64(connections)
	}
	if bytes <= 0 {
		return 0
	}
	if protocol != ProtocolUDP && protocol != ProtocolICMP {
		return 0
	}
	if direction == DirectionIngress || direction == DirectionEgress {
		return 1
	}
	return 0
}
