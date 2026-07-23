package service

func hiddenByGraphVisibility(visibility GraphVisibility, srcPort, dstPort int, srcIP, dstIP string) bool {
	if len(visibility.AllowedPorts) > 0 && !portMatchesAny(visibility.AllowedPorts, srcPort, dstPort) {
		return true
	}
	if portMatchesAny(visibility.ExcludedPorts, srcPort, dstPort) {
		return true
	}
	for _, ip := range visibility.ExcludedIPs {
		if ip == "" {
			continue
		}
		if srcIP == ip || dstIP == ip {
			return true
		}
	}
	return false
}

func hiddenByServicePort(visibility GraphVisibility, servicePort int) bool {
	if servicePort <= 0 {
		return false
	}
	if len(visibility.AllowedPorts) > 0 && !intIn(servicePort, visibility.AllowedPorts) {
		return true
	}
	return intIn(servicePort, visibility.ExcludedPorts)
}

func portMatchesAny(values []int, ports ...int) bool {
	for _, port := range ports {
		if intIn(port, values) {
			return true
		}
	}
	return false
}

func shouldFlipServiceResponse(srcPort, dstPort int) bool {
	return srcPort > 0 && dstPort > 0 && !isEphemeralPort(srcPort) && isEphemeralPort(dstPort)
}

func intIn(value int, values []int) bool {
	for _, item := range values {
		if value == item {
			return true
		}
	}
	return false
}
