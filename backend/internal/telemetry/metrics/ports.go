package metrics

import "fmt"

func NormalizePorts(protocol string, srcPort, dstPort int) (int, int) {
	if protocol == ProtocolICMP {
		return 0, 0
	}
	return srcPort, dstPort
}

func ValidatePortRange(srcPort, dstPort int) error {
	if srcPort < 0 || srcPort > 65535 || dstPort < 0 || dstPort > 65535 {
		return fmt.Errorf("ports must be between 0 and 65535")
	}
	return nil
}
