package network

type Direction string

const (
	DirectionUnknown  Direction = ""
	DirectionIngress  Direction = "ingress"
	DirectionEgress   Direction = "egress"
	DirectionInternal Direction = "internal"
)
