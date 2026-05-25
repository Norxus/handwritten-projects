package core

import "context"

func GetInterruptState[T any](ctx context.Context) (wasInterrupted bool, hasState bool, state T) {
	rCtx, ok := getRunCtx(ctx)
	// 这次执行不是从中断恢复来的
	if !ok || rCtx.interruptState == nil {
		return
	}

	// 一旦 interruptState 存在，就把 wasInterrupted 设为 true
	wasInterrupted = true
	// 此时表示“确实中断过，但没有状态数据”
	if rCtx.interruptState.State == nil {
		return
	}

	state, hasState = rCtx.interruptState.State.(T)
	return
}

// 当前组件是否处在一次 resume 流程里，并且是不是这次恢复路径上的目标”
func GetResumeContext[T any](ctx context.Context) (isResumeTarget bool, hasData bool, data T) {
	rCtx, ok := getRunCtx(ctx)
	if !ok {
		return
	}

	// 当前组件自己，或者它的某个子孙组件，是否是这次 Resume() / ResumeWithData() 的目标
	isResumeTarget = rCtx.isResumeTarget
	if !isResumeTarget {
		return
	}

	if rCtx.resumeData == nil {
		return
	}

	data, hasData = rCtx.resumeData.(T)
	return
}

func getRunCtx(ctx context.Context) (*addrCtx, bool) {
	rCtx, ok := ctx.Value(addrCtxKey{}).(*addrCtx)
	return rCtx, ok
}
