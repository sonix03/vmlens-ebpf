package pipeline

type Registry struct {
	enabled map[string]bool
}

func NewRegistry() *Registry {
	return &Registry{enabled: map[string]bool{}}
}

func (r *Registry) Enable(name string) {
	r.enabled[name] = true
}

func (r *Registry) Enabled(name string) bool {
	return r.enabled[name]
}
