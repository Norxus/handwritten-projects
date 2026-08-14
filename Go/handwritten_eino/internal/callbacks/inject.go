package callbacks

import (
	"context"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

func InitCallbacks(ctx context.Context, info *RunInfo, handlers ...Handler) context.Context {
	mgr, ok := newManager(info, handlers...)
	if !ok {
		return ctxWithManager(ctx, mgr)
	}
	return ctxWithManager(ctx, nil)
}

// 没有 manager 和 runInfo 就重新初始化一个，塞入 ctx 中
func EnsureRunInfo(ctx context.Context, typ string, comp components.Component) context.Context {
	cbm, ok := managerFromCtx(ctx)
	if !ok {
		return InitCallbacks(ctx, &RunInfo{
			Type:      typ,
			Component: comp,
		})
	}
	if cbm.runInfo == nil {
		return ReuseHandlers(ctx, &RunInfo{
			Type:      typ,
			Component: comp,
		})
	}
	return ctx
}

// 如果 ctx 里有 manager，那么直接用这个 manager (会进行浅复制)
func ReuseHandlers(ctx context.Context, info *RunInfo) context.Context {
	cbm, ok := managerFromCtx(ctx)
	if !ok {
		return InitCallbacks(ctx, info)
	}
	return ctxWithManager(ctx, cbm.withRunInfo(info))
}

type Handle[T any] func(context.Context, T, *RunInfo, []Handler) (context.Context, T)

// 接收一个 ctx 、一份当前要传给回调处理的 inOut 、一个真正执行回调逻辑的 handle 、一个用于筛选 handler 的 timing ，
// 以及一个表示这是不是“开始阶段”的 start ，然后把合适的回调 handler 组织好，最后交给 handle 去执行
func On[T any](ctx context.Context, inOut T, handle Handle[T], timing CallbackTiming, start bool) (context.Context, T) {
	mgr, ok := managerFromCtx(ctx)
	if !ok {
		return ctx, inOut
	}
	nMgr := *mgr

	var info *RunInfo
	// start = true，表示这是一次调用链的开始
	if start {
		info = nMgr.runInfo
		// 这里清理 runInfo
		nMgr.runInfo = nil
		// runInfo 传递
		ctx = context.WithValue(ctx, CtxRunInfoKey{}, info)
	} else {
		if nMgr.runInfo != nil {
			info = nMgr.runInfo
		} else {
			info, _ = ctx.Value(CtxRunInfoKey{}).(*RunInfo)
		}
	}

	hs := make([]Handler, 0, len(nMgr.handlers)+len(nMgr.globalHandlers))
	// 判断当前时机要不要执行
	for _, handler := range append(nMgr.handlers, nMgr.globalHandlers...) {
		timingChecker, ok_ := handler.(TimingChecker)
		if !ok_ || timingChecker.Needed(ctx, info, timing) {
			hs = append(hs, handler)
		}
	}

	var out T
	ctx, out = handle(ctx, inOut, info, hs)
	return ctxWithManager(ctx, &nMgr), out
}

// 依次调用 handlers 的 OnStart 方法
func OnStartHandle[T any](ctx context.Context, input T, runInfo *RunInfo, handlers []Handler) (context.Context, T) {
	for i := len(handlers) - 1; i >= 0; i-- {
		ctx = handlers[i].OnStart(ctx, runInfo, input)
	}

	return ctx, input
}

// 依次调用 handlers 的 OnEnd 方法
func OnEndHandle[T any](ctx context.Context, output T, runInfo *RunInfo, handlers []Handler) (context.Context, T) {
	for _, handler := range handlers {
		ctx = handler.OnEnd(ctx, runInfo, output)
	}

	return ctx, output
}

// 给每个 OnEnd handler 分发独立输出副本”的工厂函数
func BuildOnEndHandleWithCopy[T any](copyFn func(T, int) []T) Handle[T] {
	return func(ctx context.Context, output T, runInfo *RunInfo, handlers []Handler) (context.Context, T) {
		if len(handlers) == 0 {
			return ctx, output
		}

		copies := copyFn(output, len(handlers))

		for i, handler := range handlers {
			ctx = handler.OnEnd(ctx, runInfo, copies[i])
		}

		return ctx, output
	}
}

// 把一个流同时分发给多个 callback handler，
// 处理逻辑交给 handle
// 并把最后留给主流程继续使用的那份流返回出去
func OnWithStreamHandle[S any](
	ctx context.Context,
	inOut S,
	handlers []Handler,
	cpy func(int) []S,
	handle func(context.Context, Handler, S) context.Context) (context.Context, S) {

	if len(handlers) == 0 {
		return ctx, inOut
	}

	// 额外多复制了一个，最后那一份不会给 handler
	inOuts := cpy(len(handlers) + 1)

	for i, handler := range handlers {
		ctx = handle(ctx, handler, inOuts[i])
	}

	// 经过所有 handler 更新后的 ctx
	// 最后那份没给 handler 的流副本
	return ctx, inOuts[len(inOuts)-1]
}

// 用 handlers 处理输入流 input
func OnStartWithStreamInputHandle[T any](ctx context.Context, input *schema.StreamReader[T],
	runInfo *RunInfo, handlers []Handler) (context.Context, *schema.StreamReader[T]) {
	// OnStart 类回调按“后注册先执行”的顺序跑，类似于中间件
	handlers = generic.Reverse(handlers)

	cpy := input.Copy

	handle := func(ctx context.Context, handler Handler, in *schema.StreamReader[T]) context.Context {
		// 这里的 convert 是直接返回，所以相当于只是做了一个类型适配
		in_ := schema.StreamReaderWithConvert(in, func(i T) (CallbackInput, error) {
			return i, nil
		})
		return handler.OnStartWithStreamInput(ctx, runInfo, in_)
	}

	return OnWithStreamHandle(ctx, input, handlers, cpy, handle)
}

// 用 handlers 处理输出流 output
func OnEndWithStreamOutputHandle[T any](ctx context.Context, output *schema.StreamReader[T],
	runInfo *RunInfo, handlers []Handler) (context.Context, *schema.StreamReader[T]) {
	cpy := output.Copy

	handle := func(ctx context.Context, handler Handler, out *schema.StreamReader[T]) context.Context {
		out_ := schema.StreamReaderWithConvert(out, func(i T) (CallbackOutput, error) {
			return i, nil
		})
		return handler.OnEndWithStreamOutput(ctx, runInfo, out_)
	}

	return OnWithStreamHandle(ctx, output, handlers, cpy, handle)
}

// 依次调用 handlers 的 OnError
func OnErrorHandle(ctx context.Context, err error, runInfo *RunInfo, handlers []Handler) (context.Context, error) {
	for _, handler := range handlers {
		ctx = handler.OnError(ctx, runInfo, err)
	}

	return ctx, err
}
