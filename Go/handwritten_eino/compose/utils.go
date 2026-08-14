package compose

import (
	"context"

	"github.com/cloudwego/eino/callbacks"
	icb "github.com/cloudwego/eino/internal/callbacks"
	"github.com/cloudwego/eino/schema"
)

type on[T any] func(context.Context, T) (context.Context, T)

// 调用注册在 Start 时机的 handler 的 onStart 方法
func onStart[T any](ctx context.Context, input T) (context.Context, T) {
	return icb.On(ctx, input, icb.OnStartHandle[T], callbacks.TimingOnStart, true)
}

func onEnd[T any](ctx context.Context, output T) (context.Context, T) {
	return icb.On(ctx, output, icb.OnEndHandle[T], callbacks.TimingOnEnd, false)
}

func onError(ctx context.Context, err error) (context.Context, error) {
	return icb.On(ctx, err, icb.OnErrorHandle, callbacks.TimingOnError, false)
}

func onStartWithStreamInput[T any](ctx context.Context, input *schema.StreamReader[T]) (
	context.Context, *schema.StreamReader[T]) {
	return icb.On(ctx, input, icb.OnStartWithStreamInputHandle[T], callbacks.TimingOnStartWithStreamInput, true)
}

func onEndWithStreamOutput[T any](ctx context.Context, output *schema.StreamReader[T]) (
	context.Context, *schema.StreamReader[T]) {
	return icb.On(ctx, output, icb.OnEndWithStreamOutputHandle[T], callbacks.TimingOnEndWithStreamOutput, false)
}

// 接收一个原始执行函数 r 以及 3 种生命周期钩子，最后返回一个包装函数，将这 3 个生命周期钩子插入到 r 的执行过程里
func runWithCallbacks[I, O, TOption any](r func(context.Context, I, ...TOption) (O, error),
	onStart on[I], onEnd on[O], onError on[error]) func(context.Context, I, ...TOption) (O, error) {
	return func(ctx context.Context, input I, opts ...TOption) (output O, err error) {
		ctx, input = onStart(ctx, input)

		output, err = r(ctx, input, opts...)
		if err != nil {
			ctx, err = onError(ctx, err)
			return output, err
		}

		ctx, output = onEnd(ctx, output)

		return output, nil
	}
}

func invokeWithCallbacks[I, O, TOption any](i Invoke[I, O, TOption]) Invoke[I, O, TOption] {
	return runWithCallbacks(i, onStart[I], onEnd[O], onError)
}

func streamWithCallbacks[I, O, TOption any](s Stream[I, O, TOption]) Stream[I, O, TOption] {
	return runWithCallbacks(s, onStart[I], onEndWithStreamOutput[O], onError)
}

func collectWithCallbacks[I, O, TOption any](c Collect[I, O, TOption]) Collect[I, O, TOption] {
	return runWithCallbacks(c, onStartWithStreamInput[I], onEnd[O], onError)
}

func transformWithCallbacks[I, O, TOption any](t Transform[I, O, TOption]) Transform[I, O, TOption] {
	return runWithCallbacks(t, onStartWithStreamInput[I], onEndWithStreamOutput[O], onError)
}
