package compose

type Chain[I, O any] struct {
	err         error
	gg          *Graph[I, O]
	nodeIdx     int
	preNodeKeys []string
	hasEnd      bool
}

func NewChain[I, O any](opts ...NewGraphOption) *Chain[I, O] {
	ch := &Chain[I, O]{}
}
