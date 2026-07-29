package packet

func DecodeRawEvent(sample []byte) (RawEvent, error) {
	return decodeRawFlowEvent(sample)
}
