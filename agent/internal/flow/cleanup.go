package flow

func (a *Accumulator) Reset() {
	if a == nil {
		return
	}
	a.flows = make(map[Key]*State)
}
