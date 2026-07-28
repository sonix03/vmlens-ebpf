package metrics

func NormalizePorts(protocol string, srcPort, dstPort int) (int, int) {
	if protocol == ProtocolICMP {
		return 0, 0
	}
	return srcPort, dstPort
}
