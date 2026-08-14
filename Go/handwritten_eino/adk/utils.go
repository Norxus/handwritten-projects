package adk

import "github.com/cloudwego/eino/internal"

type AsyncIterator[T any] struct {
	ch *internal.UnboundedChan[T]
}