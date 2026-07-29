package packets

func Apply(model *Model, count uint64) {
	if model == nil {
		return
	}
	model.Count += count
}
