package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/internal/core"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

var deprecatedInterruptAndRerun = errors.New("interrupt and rerun")

type InterruptCtx = core.InterruptCtx

type AddressSegment = core.AddressSegment

func IsInterruptRerunError(err error) (any, bool) {
	info, _, ok := isInterruptRerunError(err)
	return info, ok
}

func isInterruptRerunError(err error) (info any, state any, ok bool) {
	// 兼容老板本的错误，老板本的错误没有 info, state 信息
	if errors.Is(err, deprecatedInterruptAndRerun) {
		return nil, nil, true
	}
	ire := &core.InterruptSignal{}
	// 兼容新版本的错误
	if errors.As(err, &ire) {
		return ire.Info, ire.State, true
	}
	return nil, nil, false
}

type InterruptInfo struct {
	State            any
	BeforeNodes      []string
	AfterNodes       []string
	RerunNodes       []string
	RerunNodesExtra  map[string]any
	SubGraphs        map[string]*InterruptInfo
	InterruptContexts []*InterruptCtx
}

type AddressSegmentType = core.AddressSegmentType

type Address = core.Address

const (
	AddressSegmentNode     AddressSegmentType = "node"
	AddressSegmentTool     AddressSegmentType = "tool"
	AddressSegmentRunnable AddressSegmentType = "runnable"
)

func init() {
	schema.RegisterName[*InterruptInfo]("_eino_compose_interrupt_info")
}

func GetCurrentAddress(ctx context.Context) Address {
	return core.GetCurrentAddress(ctx)
}

type wrappedInterruptAndRerun struct {
	ps    Address
	inner error
}

func (w *wrappedInterruptAndRerun) Error() string {
	return fmt.Sprintf("interrupt and rerun at address %s: %s", w.ps.String(), w.inner.Error())
}

func (w *wrappedInterruptAndRerun) Unwrap() error {
	return w.inner
}

type interruptError struct {
	Info *InterruptInfo
}

func (e *interruptError) Error() string {
	return fmt.Sprintf("interrupt happened, info: %+v", e.Info)
}

func WrapInterruptAndRerunIfNeeded(ctx context.Context, step AddressSegment, err error) error {
	// 当前地址加上马上要走的 step 等于新地址
	addr := GetCurrentAddress(ctx)
	newAddr := append(append([]AddressSegment{}, addr...), step)
	// 如果 err 是旧版的 deprecatedInterruptAndRerun
	// 就包装成 wrappedInterruptAndRerun，把地址 newAddr 塞进去
	if errors.Is(err, deprecatedInterruptAndRerun) {
		return &wrappedInterruptAndRerun{
			ps:    newAddr,
			inner: err,
		}
	}

	// 如果 err 是新的 core.InterruptSignal
	ire := &core.InterruptSignal{}
	if errors.As(err, &ire) {
		// 没有地址就包装地址
		if ire.Address == nil {
			return &wrappedInterruptAndRerun{
				ps:    newAddr,
				inner: err,
			}
		}
		return ire
	}

	return fmt.Errorf("failed to wrap error as addressed InterruptAndRerun: %w", err)
}

// 主动中断当前 graph/component 的执行，并把一份内部状态一起保存到 checkpoint，方便之后 resume 时恢复
func StatefulInterrupt(ctx context.Context, info any, state any) error {
	is, err := core.Interrupt(ctx, info, state, nil)
	if err != nil {
		return err
	}
	return is
}

// 一个父组件内部可能有多个子任务同时中断，需要把这些子中断合并成一个 error 返回给 graph 引擎
// ToolsNode
//├── tool A 中断
//├── tool B 成功
//└── tool C 中断
// ToolsNode 最终只能向外返回一个 error，但里面实际有两个中断点：tool A 和 tool C
// CompositeInterrupt 就是把这些子 error 收集起来，构造成一棵 InterruptSignal 树
func CompositeInterrupt(ctx context.Context, info any, state any, errs ...error) error {
	// 没有子中断，那就退化成当前组件自己的 stateful interrupt
	if len(errs) == 0 {
		return StatefulInterrupt(ctx, info, state)
	}

	// 遍历所有子错误 errs，把它们统一转成 *core.InterruptSignal
	var cErrs []*core.InterruptSignal
	for _, err := range errs {
		// 接下来就是处理不同类型的错误
		wrapped := &wrappedInterruptAndRerun{}
		if errors.As(err, &wrapped) {
			inner := wrapped.Unwrap()
			if errors.Is(inner, deprecatedInterruptAndRerun) {
				id := uuid.NewString()
				cErrs = append(cErrs, &core.InterruptSignal{
					ID:      id,
					Address: wrapped.ps,
					InterruptInfo: core.InterruptInfo{
						Info:        nil,
						IsRootCause: true,
					},
				})
				continue
			}

			ire := &core.InterruptSignal{}
			if errors.As(err, &ire) {
				id := uuid.NewString()
				cErrs = append(cErrs, &core.InterruptSignal{
					ID:      id,
					Address: wrapped.ps,
					InterruptInfo: core.InterruptInfo{
						Info:        ire.InterruptInfo.Info,
						IsRootCause: ire.InterruptInfo.IsRootCause,
					},
					InterruptState: core.InterruptState{
						State: ire.InterruptState.State,
					},
				})
			}

			continue
		}

		ire := &core.InterruptSignal{}
		if errors.As(err, &ire) {
			cErrs = append(cErrs, ire)
			continue
		}

		ie := &interruptError{}
		if errors.As(err, &ie) {
			is := core.FromInterruptContexts(ie.Info.InterruptContexts)
			cErrs = append(cErrs, is)
			continue
		}

		// 如果某个子 error 不是 interrupt 类型，直接报错
		return fmt.Errorf("composite interrupt but one of the sub error is not interrupt and rerun error: %w", err)
		
	}

	// 上面生成了子中断错误，这里调用 interrupt，把当前节点的信息也包进去
	// 最终返回完整的中断错误
	is, err := core.Interrupt(ctx, info, state, cErrs)
	if err != nil {
		return err
	}

	return is
}
