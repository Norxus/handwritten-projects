package pool

import (
	"context"
	"sync"

	conc "code.byted.org/aweme/conc_test"
)

func New() *Pool {
	return &Pool{}
}

type Pool struct {
	handle   conc.WaitGroup
	limiter  limiter
	tasks    chan func()
	initOnce sync.Once
}

func (p *Pool) Go(f func()) {
	p.init()

	if p.limiter == nil {
		select {
		case p.tasks <- f:
		default:
			p.handle.Go(func() {
				p.worker(f)
			})
		}
	} else {
		select {
		case p.limiter <- struct{}{}:
			p.handle.Go(func() {
				p.worker(f)
			})
		case p.tasks <- f:
			return
		}
	}
}

func (p *Pool) Wait() {
	p.init()
	close(p.tasks)
	defer func() {
		p.initOnce = sync.Once{}
	}()

	p.handle.Wait()
}

func (p *Pool) MaxGoroutines() int {
	return p.limiter.limit()
}

func (p *Pool) WithMaxGoroutines(n int) *Pool {
	p.panicIfInitialized()
	if n < 1 {
		panic("max goroutines in a pool must be greater than zero")
	}
	p.limiter = make(limiter, n)
	return p
}

func (p *Pool) WithContext(ctx context.Context) *ContextPool {
	p.panicIfInitialized()
	ctx, cancel := context.WithCancel(ctx)
	return &ContextPool{
		errorPool: p.WithErrors().deref(),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (p *Pool) panicIfInitialized() {
	if p.tasks != nil {
		panic("pool can not be reconfigured after calling Go() for the first time")
	}
}

func (p *Pool) WithErrors() *ErrorPool {
	p.panicIfInitialized()
	return &ErrorPool{
		pool: p.deref(),
	}
}

func (p *Pool) deref() Pool {
	p.panicIfInitialized()
	return Pool{
		limiter: p.limiter,
	}
}

func (p *Pool) worker(initialFunc func()) {
	defer p.limiter.release()
	if initialFunc != nil {
		initialFunc()
	}

	for f := range p.tasks {
		f()
	}
}

func (p *Pool) init() {
	p.initOnce.Do(func() {
		p.tasks = make(chan func())
	})
}

type limiter chan struct{}

func (l limiter) limit() int {
	return cap(l)
}

func (l limiter) release() {
	if l != nil {
		<-l
	}
}
