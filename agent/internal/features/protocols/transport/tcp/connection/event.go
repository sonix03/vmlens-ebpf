package connection

type Event struct {
	Protocol    string
	Direction   string
	Bytes       int64
	Connections uint32
	Errors      uint32
}
