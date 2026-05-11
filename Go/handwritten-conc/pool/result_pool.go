package pool

func NewWithResults[T any]() *ResultPool[T] {
	return &ResultPool[T]{
		pool: *New(),
	}

}

type ResultPool[T any] struct {
	pool Pool
	agg  resultAggregator[T]
}

func (p *ResultPool[T]) Go(f func() T) {
	idx := p.agg.nextIndex()
	p.pool.Go(func() {
		p.agg.save(idx, f(), false)
	})
}

func (p *ResultPool[T]) Wait() []T {
	p.pool.Wait()
	results := p.agg.collect(true)
	p.agg = resultAggregator[T]{}
	return results
}

func (p *ResultPool[T]) MaxGoroutines() int {
	return p.pool.MaxGoroutines()
}

func (p *ResultPool[T]) panicIfInitialized() {
	p.pool.panicIfInitialized()
}
