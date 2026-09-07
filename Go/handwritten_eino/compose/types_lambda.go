package compose

import (
	"context"
	"github.com/cloudwego/eino/schema"
)

type Invoke[I, O, TOption any] func(ctx context.Context, input I, options ...TOption) (output O, err error)

type Stream[I, O, TOption any] func(ctx context.Context, input I, opts ...TOption) (output *schema.StreamReader[O], err error)

type Collect[I, O, TOption any] func(ctx context.Context, input *schema.StreamReader[I], opts ...TOption) (output O, err error)

type Transform[I, O, TOption any] func(ctx context.Context, input *schema.StreamReader[I], opts ...TOption) (output *schema.StreamReader[O], err error)

type Lambda struct {
	executor *composableRunnable
}
