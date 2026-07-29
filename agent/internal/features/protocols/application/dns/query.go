package dns

func NewQuery(name string, queryType uint16) Query {
	return Query{Name: name, Type: queryType}
}
