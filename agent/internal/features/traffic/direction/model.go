package direction

const (
	Unknown = "unknown"
	Ingress = "ingress"
	Egress  = "egress"
)

type Model struct {
	Current string
}
