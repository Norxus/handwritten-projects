package callbacks

import (
	"context"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/internal/callbacks"
	"github.com/cloudwego/eino/schema"
)

func EnsureRunInfo(ctx context.Context, typ string, comp components.Component) context.Context {
	return callbacks.EnsureRunInfo(ctx, typ, comp)
}

func OnStart[T any](ctx context.Context, input T) context.Context {
	ctx, _ = callbacks.On(ctx, input, callbacks.OnStartHandle[T], TimingOnStart, true)
	return ctx
}

func OnError(ctx context.Context, err error) context.Context {
	ctx, _ = callbacks.On(ctx, err, callbacks.OnErrorHandle, TimingOnError, false)
	return ctx
}

func OnEnd[T any](ctx context.Context, output T) context.Context {
	ctx, _ = callbacks.On(ctx, output, callbacks.OnEndHandle[T], TimingOnEnd, false)
	return ctx
}

func OnStartWithStreamInput[T any](ctx context.Context, input *schema.StreamReader[T]) (
	nextCtx context.Context, newStreamReader *schema.StreamReader[T]) {
	return callbacks.On(ctx, input, callbacks.OnStartWithStreamInputHandle[T], TimingOnStartWithStreamInput, true)
}

func OnEndWithStreamOutput[T any](ctx context.Context, output *schema.StreamReader[T]) (
	nextCtx context.Context, newStreamReader *schema.StreamReader[T]) {
	return callbacks.On(ctx, output, callbacks.OnEndWithStreamOutputHandle[T], TimingOnEndWithStreamOutput, true)
}
