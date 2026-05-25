package callbacks

import (
	"context"

	"github.com/cloudwego/eino/components"
)

func InitCallbacks(ctx context.Context, info *RunInfo, handlers ...Handler) context.Context {
	mgr, ok := newManager(info, handlers...)
	if !ok {
		return ctxWithManager(ctx, mgr)
	}
	return ctxWithManager(ctx, nil)
}

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
