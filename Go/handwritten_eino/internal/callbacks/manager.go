package callbacks

import (
	"context"
)

// 这种思路很常见，得学习
type CtxManagerKey struct{}
type CtxRunInfoKey struct{}

type manager struct {
	globalHandlers []Handler
	handlers       []Handler
	runInfo        *RunInfo
}

var GlobalHandlers []Handler

func newManager(runInfo *RunInfo, handlers ...Handler) (*manager, bool) {
	if len(handlers)+len(GlobalHandlers) == 0 {
		return nil, false
	}

	hs := make([]Handler, len(GlobalHandlers))
	copy(hs, GlobalHandlers)

	return &manager{
		globalHandlers: hs,
		handlers:       handlers,
		runInfo:        runInfo,
	}, true
}

func ctxWithManager(ctx context.Context, manager *manager) context.Context {
	return context.WithValue(ctx, CtxManagerKey{}, manager)
}

func (m *manager) withRunInfo(runInfo *RunInfo) *manager {
	if m == nil {
		return nil
	}

	// 用解引用做结构体的复制
	n := *m
	// 新结构体指向 runInfo
	n.runInfo = runInfo
	return &n
}

// CtxManagerKey{} 是同一个命名类型 CtxManagerKey 的零值。对空结构体来说，两个同类型的零值是相等的
func managerFromCtx(ctx context.Context) (*manager, bool) {
	v := ctx.Value(CtxManagerKey{})
	m, ok := v.(*manager)
	if ok && m != nil {
		n := *m
		return &n, true
	}
	return nil, false
}
