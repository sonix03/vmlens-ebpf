package icmp

func Apply(model *Model, event Event) {
	if model == nil {
		return
	}
	model.Requests++
	if event.Bytes > 0 {
		model.Bytes += uint64(event.Bytes)
	}
}
