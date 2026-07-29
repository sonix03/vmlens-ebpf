package connection

type Model struct {
	OpenCount   uint64
	CloseCount  uint64
	ErrorCount  uint64
	RequestHint uint64
}
