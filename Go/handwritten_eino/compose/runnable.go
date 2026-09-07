package compose

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

type runnablePacker[I, O, TOption any] struct {
	i Invoke[I, O, TOption]
	s Stream[I, O, TOption]
	c Collect[I, O, TOption]
	t Transform[I, O, TOption]
}

type invoke func(ctx context.Context, input any, opts ...any) (output any, err error)
type transform func(ctx context.Context, input streamReader, opts ...any) (output streamReader, err error)

type composableRunnable struct {
	i invoke
	t transform

	inputType  reflect.Type
	outputType reflect.Type
	optionType reflect.Type

	*genericHelper

	isPassthrough bool
	meta          *executorMeta

	nodeInfo *nodeInfo
}

// 把组件暴露出来的几种执行方法，包装成框架内部统一的 *composableRunnable
func runnableLambda[I, O, TOption any](i Invoke[I, O, TOption], s Stream[I, O, TOption], c Collect[I, O, TOption],
	t Transform[I, O, TOption], enableCallback bool) *composableRunnable {
	rp := newRunnablePacker(i, s, c, t, enableCallback)
	return rp.toComposableRunnable()
}

// 桥接
func (rp *runnablePacker[I, O, TOption]) Invoke(ctx context.Context,
	input I, opts ...TOption) (output O, err error) {
	return rp.i(ctx, input, opts...)
}

func (rp *runnablePacker[I, O, TOption]) Stream(ctx context.Context,
	input I, opts ...TOption) (output *schema.StreamReader[O], err error) {

	return rp.s(ctx, input, opts...)
}

func (rp *runnablePacker[I, O, TOption]) Collect(ctx context.Context,
	input *schema.StreamReader[I], opts ...TOption) (output O, err error) {
	return rp.c(ctx, input, opts...)
}

func (rp *runnablePacker[I, O, TOption]) Transform(ctx context.Context,
	input *schema.StreamReader[I], opts ...TOption) (output *schema.StreamReader[O], err error) {
	return rp.t(ctx, input, opts...)
}

// 给传进来的 4 种能力形态里，补齐剩下那几种，最后组装成一个完整的 runnablePacker
func newRunnablePacker[I, O, TOption any](i Invoke[I, O, TOption], s Stream[I, O, TOption],
	c Collect[I, O, TOption], t Transform[I, O, TOption], enableCallback bool) *runnablePacker[I, O, TOption] {
	r := &runnablePacker[I, O, TOption]{}

	// 如果开启 callbak，那么给传入的函数加上指定的 callback
	if enableCallback {
		if i != nil {
			i = invokeWithCallbacks(i)
		}

		if s != nil {
			s = streamWithCallbacks(s)
		}

		if c != nil {
			c = collectWithCallbacks(c)
		}

		if t != nil {
			t = transformWithCallbacks(t)
		}
	}

	// 如果传入了 i 直接用，如果没有传入，那么尝试从其他传入的函数转换过来
	if i != nil {
		r.i = i
	} else if s != nil {
		r.i = invokeByStream(s)
	} else if c != nil {
		r.i = invokeByCollect(c)
	} else {
		r.i = invokeByTransform(t)
	}

	if s != nil {
		r.s = s
	} else if t != nil {
		r.s = streamByTransform(t)
	} else if i != nil {
		r.s = streamByInvoke(i)
	} else {
		r.s = streamByCollect(c)
	}

	if c != nil {
		r.c = c
	} else if t != nil {
		r.c = collectByTransform(t)
	} else if i != nil {
		r.c = collectByInvoke(i)
	} else {
		r.c = collectByStream(s)
	}

	if t != nil {
		r.t = t
	} else if s != nil {
		r.t = transformByStream(s)
	} else if c != nil {
		r.t = transformByCollect(c)
	} else {
		r.t = transformByInvoke(i)
	}

	return r
}

// 把流式结果合并得到一个最终结果返回
func invokeByStream[I, O, TOption any](s Stream[I, O, TOption]) Invoke[I, O, TOption] {
	return func(ctx context.Context, input I, opts ...TOption) (output O, err error) {
		// 获得一个 *schema.StreamReader[O]
		sr, err := s(ctx, input, opts...)
		if err != nil {
			return output, err
		}
		// StreamReader[O] -> O
		return defaultImplConcatStreamReader(sr)
	}
}

func invokeByCollect[I, O, TOption any](c Collect[I, O, TOption]) Invoke[I, O, TOption] {
	return func(ctx context.Context, input I, opts ...TOption) (output O, err error) {
		// 把普通输入 input I 包成一个只含一个元素的流，从而适配只接受流的函数
		sr := schema.StreamReaderFromArray([]I{input})
		return c(ctx, sr, opts...)
	}
}

// 单值 -> 流输入 -> transform -> 流输出 -> 合并得到单一结果
func invokeByTransform[I, O, TOption any](t Transform[I, O, TOption]) Invoke[I, O, TOption] {
	return func(ctx context.Context, input I, opts ...TOption) (output O, err error) {
		srInput := schema.StreamReaderFromArray([]I{input})

		srOutput, err := t(ctx, srInput, opts...)
		if err != nil {
			return output, err
		}

		return defaultImplConcatStreamReader(srOutput)
	}
}

func defaultImplConcatStreamReader[T any](
	sr *schema.StreamReader[T]) (T, error) {

	c, err := concatStreamReader(sr)
	if err != nil {
		var t T
		return t, fmt.Errorf("concat stream reader fail: %w", err)
	}

	return c, nil
}

// 把流通过 Transform 得到另外一个输出流
func streamByTransform[I, O, TOption any](t Transform[I, O, TOption]) Stream[I, O, TOption] {
	return func(ctx context.Context, input I, opts ...TOption) (output *schema.StreamReader[O], err error) {
		// 先转换为流
		srInput := schema.StreamReaderFromArray([]I{input})

		// 经过转换逻辑，实现 I 流到 O 流的转换
		return t(ctx, srInput, opts...)
	}
}

// 把 invoke 的结果包装成一个输出流
func streamByInvoke[I, O, TOption any](i Invoke[I, O, TOption]) Stream[I, O, TOption] {
	return func(ctx context.Context, input I, opts ...TOption) (output *schema.StreamReader[O], err error) {
		// 先使用 Invoke 产生一个值
		out, err := i(ctx, input, opts...)
		if err != nil {
			return nil, err
		}

		// 把值变成流
		return schema.StreamReaderFromArray([]O{out}), nil
	}
}

// 单值输入 -> 流 -> collect 为单值 -> 流
func streamByCollect[I, O, TOption any](c Collect[I, O, TOption]) Stream[I, O, TOption] {
	return func(ctx context.Context, input I, opts ...TOption) (output *schema.StreamReader[O], err error) {
		srInput := schema.StreamReaderFromArray([]I{input})

		out, err := c(ctx, srInput, opts...)
		if err != nil {
			return nil, err
		}

		return schema.StreamReaderFromArray([]O{out}), nil
	}
}

// 流输入 -> 流输出 -> 合并得到单值结果
func collectByTransform[I, O, TOption any](t Transform[I, O, TOption]) Collect[I, O, TOption] {
	return func(ctx context.Context, input *schema.StreamReader[I], opts ...TOption) (output O, err error) {
		srOutput, err := t(ctx, input, opts...)
		if err != nil {
			return output, err
		}

		return defaultImplConcatStreamReader(srOutput)
	}
}

// 输入流 -> 单值 -> 单值
func collectByInvoke[I, O, TOption any](i Invoke[I, O, TOption]) Collect[I, O, TOption] {
	return func(ctx context.Context, input *schema.StreamReader[I], opts ...TOption) (output O, err error) {
		in, err := defaultImplConcatStreamReader(input)
		if err != nil {
			return output, err
		}

		return i(ctx, in, opts...)
	}
}

// 输入流 -> 单值 -> 输出流 ->单值
func collectByStream[I, O, TOption any](s Stream[I, O, TOption]) Collect[I, O, TOption] {
	return func(ctx context.Context, input *schema.StreamReader[I], opts ...TOption) (output O, err error) {
		in, err := defaultImplConcatStreamReader(input)
		if err != nil {
			return output, err
		}

		srOutput, err := s(ctx, in, opts...)
		if err != nil {
			return output, err
		}

		return defaultImplConcatStreamReader(srOutput)
	}
}

// 输入流 -> 单值 -> 输出流
func transformByStream[I, O, TOption any](s Stream[I, O, TOption]) Transform[I, O, TOption] {
	return func(ctx context.Context, input *schema.StreamReader[I],
		opts ...TOption) (output *schema.StreamReader[O], err error) {
		in, err := defaultImplConcatStreamReader(input)
		if err != nil {
			return output, err
		}

		return s(ctx, in, opts...)
	}
}

// 输入流 -> 单值 -> 输出流
func transformByCollect[I, O, TOption any](c Collect[I, O, TOption]) Transform[I, O, TOption] {
	return func(ctx context.Context, input *schema.StreamReader[I],
		opts ...TOption) (output *schema.StreamReader[O], err error) {
		out, err := c(ctx, input, opts...)
		if err != nil {
			return output, err
		}

		return schema.StreamReaderFromArray([]O{out}), nil
	}
}

// 输入流 ->单值 ——> 单值 -> 输出流
func transformByInvoke[I, O, TOption any](i Invoke[I, O, TOption]) Transform[I, O, TOption] {
	return func(ctx context.Context, input *schema.StreamReader[I], opts ...TOption) (output *schema.StreamReader[O], err error) {
		in, err := defaultImplConcatStreamReader(input)
		if err != nil {
			return output, err
		}

		out, err := i(ctx, in, opts...)
		if err != nil {
			return output, err
		}

		return schema.StreamReaderFromArray([]O{out}), nil
	}
}

// 把 runnablePacker 转换为 composableRunnable
func (rp *runnablePacker[I, O, TOption]) toComposableRunnable() *composableRunnable {
	// 获取运行时类型
	inputType := generic.TypeOf[I]()
	outputType := generic.TypeOf[O]()
	optionType := generic.TypeOf[TOption]()

	// 把运行时类型记录到 composableRunnble 里
	c := &composableRunnable{
		genericHelper: newGenericHelper[I, O](),
		inputType:     inputType,
		outputType:    outputType,
		optionType:    optionType,
	}

	// 转换 invoke
	i := func(ctx context.Context, input any, opts ...any) (output any, err error) {
		in, ok := input.(I)
		if !ok {
			if input == nil && reflect.TypeOf((*I)(nil)).Elem().Kind() == reflect.Interface {
				var i I
				in = i
			} else {
				panic(newUnexpectedInputTypeErr(inputType, reflect.TypeOf(input)))
			}
		}

		tos, err := convertOption[TOption](opts...)
		if err != nil {
			return nil, err
		}

		return rp.Invoke(ctx, in, tos...)
	}

	t := func(ctx context.Context, input streamReader, opts ...any) (output streamReader, err error) {
		in, ok := unpackStreamReader[I](input)
		if !ok {
			panic(newUnexpectedInputTypeErr(reflect.TypeOf(in), input.getType()))
		}

		tos, err := convertOption[TOption](opts...)
		if err != nil {
			return nil, err
		}

		out, err := rp.Transform(ctx, in, tos...)
		if err != nil {
			return nil, err
		}

		return packStreamReader(out), nil
	}

	// 之所以这里只记录 invoke 和 transform，是因为可以通过这两函数
	// 反推剩下的
	c.i = i
	c.t = t

	return c
}

// 透传节点
func composablePassthrough() *composableRunnable {
	r := &composableRunnable{isPassthrough: true, nodeInfo: &nodeInfo{}}

	// 啥也不干直接返回
	r.i = func(ctx context.Context, input any, opts ...any) (output any, err error) {
		return input , nil
	}

	r.t = func(ctx context.Context, input streamReader, opts ...any) (output streamReader, err error) {
		return input, nil
	}

	// 标记自己是 Passthrough
	r.meta = &executorMeta{
		component: ComponentOfPassthrough,
		isComponentCallbackEnabled: false,
		componentImplType: "Passthrough",
	}

	return r
}

