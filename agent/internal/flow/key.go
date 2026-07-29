package flow

type Key struct {
	AgentID   string
	SrcIP     string
	DstIP     string
	DstPort   int
	Protocol  string
	Direction string
	Interface string
}
