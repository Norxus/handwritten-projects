package pool

import (
	"context"
	"sort"
	"sync"
)

type ResultContextPool[T any] struct {
	ContextPool    ContextPool
	agg            resultAggregator[T]
	collectErrored bool
}

type resultAggregator[T any] struct {
	mu      sync.Mutex
	len     int
	results []T
	errored []int
}

func (p *ResultContextPool[T]) Go(f func(ctx context.Context) (T, error)) {
	idx := p.agg.nextIndex()
	p.ContextPool.Go(func(ctx context.Context) error {
		res, err := f(ctx)
		p.agg.save(idx, res, err != nil)
		return err
	})
}

func (p *ResultContextPool[T]) Wait() ([]T, error) {
	err := p.ContextPool.Wait()
	results := p.agg.collect(p.collectErrored)
	p.agg = resultAggregator[T]{}
	return results, err
}

func (p *ResultContextPool[T]) WithCollectErrored() *ResultContextPool[T] {
	p.panicIfInitialized()
	p.collectErrored = true
	return p
}

func (p *ResultContextPool[T]) WithFirstError() *ResultContextPool[T] {
	p.panicIfInitialized()
	p.ContextPool.WithFirstError()
	return p
}

func (p *ResultContextPool[T]) WithCancelOnError() *ResultContextPool[T] {
	p.panicIfInitialized()
	p.ContextPool.WithCancelOnError()
	return p
}

func (p *ResultContextPool[T]) WithFailFast() *ResultContextPool[T] {
	p.panicIfInitialized()
	p.ContextPool.WithFailFast()
	return p
}

func (p *ResultContextPool[T]) WithMaxGoroutines(n int) *ResultContextPool[T] {
	p.panicIfInitialized()
	p.ContextPool.WithMaxGoroutines(n)
	return p
}

func (p *ResultContextPool[T]) panicIfInitialized() {
	p.ContextPool.panicIfInitialized()
}
func (r *resultAggregator[T]) nextIndex() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	nextIdx := r.len
	r.len += 1
	return nextIdx
}

func (r *resultAggregator[T]) save(i int, res T, errored bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if i >= len(r.results) {
		old := r.results
		r.results = make([]T, r.len)
		copy(r.results, old)
	}

	r.results[i] = res

	if errored {
		r.errored = append(r.errored, i)
	}
}

func (r *resultAggregator[T]) collect(collectErrored bool) []T {
	if !r.mu.TryLock() {
		panic("collect should not be called until all goroutines have exited")
	}

	if collectErrored || len(r.errored) == 0 {
		return r.results
	}

	filtered := r.results[:0]
	sort.Ints(r.errored)
	for i, e := range r.errored {
		if i == 0 {
			filtered = append(filtered, r.results[:e]...)
		} else {
			filtered = append(filtered, r.results[r.errored[i-1]+1:e]...)
		}
	}
	return filtered
}
