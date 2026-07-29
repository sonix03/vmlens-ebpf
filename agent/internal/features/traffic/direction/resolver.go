package direction

func FromKernel(value uint8) string {
	if value == 1 {
		return Ingress
	}
	return Egress
}
