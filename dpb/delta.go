package dpb

func NewDelta(es ...*Entry) *Delta {
	return Delta_builder{Entries: es}.Build()
}
