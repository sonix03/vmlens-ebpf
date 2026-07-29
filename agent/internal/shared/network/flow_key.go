package network

type FlowKey struct {
	Source      Endpoint
	Destination Endpoint
	Protocol    Protocol
}
