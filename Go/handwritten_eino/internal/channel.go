package internal

import "sync"

// 自己造了一个 channel
type UnboundedChan[T any] struct {
	buffer   []T
	mutex    sync.Mutex
	notEmpty *sync.Cond
	closed   bool
}

func NewUnboundedCha[T any]() *UnboundedChan[T] {
	// Go 会把所有字段自动设为各自类型的零值，所以
	ch := &UnboundedChan[T]{}
	// sync.Mutex 的零值就是“未加锁的 mutex”，并且这个状态是 可直接使用 的
	// var mu sync.Mutex
	// mu.Lock()
	// mu.Unlock()
	ch.notEmpty = sync.NewCond(&ch.mutex)
	return ch
}

func (ch *UnboundedChan[T]) Send(value T) {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	if ch.closed {
		panic("send on closed channel")
	}

	// 数据放入 buffer 中
	ch.buffer = append(ch.buffer, value)
	// 告诉其他人已经有数据了
	ch.notEmpty.Signal()
}


func (ch *UnboundedChan[T]) Receive()(T, bool) {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	// 这里使用了循环，醒了之后还要检测条件
	for len(ch.buffer) == 0 && !ch.closed {
		ch.notEmpty.Wait()
	}

	// 能走到这里，说明 ch 被关了
	// 此时可能是被唤醒了走到这里
	if len(ch.buffer) == 0 {
		var zero T
		return zero, false
	}

	val := ch.buffer[0]
	ch.buffer = ch.buffer[1:]
	return val, true
}

func (ch *UnboundedChan[T]) Close() {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	// 把所有等待中的接收者叫醒，它们醒来后重新检查循环条件
	if !ch.closed {
		ch.closed = true
		ch.notEmpty.Broadcast()
	}
}
