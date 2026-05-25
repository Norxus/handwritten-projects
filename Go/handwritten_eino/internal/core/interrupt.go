package core

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/uuid"
)

type CheckPointStore interface {
	Get(ctx context.Context, checkpointID string) ([]byte, bool, error)
	Set(ctx context.Context, checkpointID string, checkPoint []byte) error
}

type InterruptSignal struct {
	ID string
	Address
	InterruptInfo
	InterruptState
	Subs []*InterruptSignal
}

func (is *InterruptSignal) Error() string {
	return fmt.Sprintf("interrupt signal: ID=%s, Addr=%s, Info=%s, State=%s, SubsLen=%d",
		is.ID, is.Address.String(), is.InterruptInfo.String, is.InterruptState.String(), len(is.subs))
}

type InterruptState struct {
	State                any
	LayerSpecificPayload any
}

func (is *InterruptState) String() string {
	if is == nil {
		return ""
	}
	return fmt.Sprintf("interrupt state: State=%v, LayerSpecificPayload=%v", is.State, is.LayerSpecificPayload)
}

type InterruptConfig struct {
	LayerPayload any
}

type InterruptOption func(*InterruptConfig)

func WithLayerPayload(payload any) InterruptOption {
	return func(c *InterruptConfig) {
		c.LayerPayload = payload
	}
}

func Interrupt(ctx context.Context, info any, state any, subContexts []*InterruptSignal, opts ...InterruptOption) (
	*InterruptSignal, error) {
	addr := GetCurrentAddress(ctx)

	config := &InterruptConfig{}
	for _, opt := range opts {
		opt(config)
	}

	myPoint := InterruptInfo{
		Info: info,
	}

	// 没有子中断，说明当前点就是中断源头
	if len(subContexts) == 0 {
		myPoint.IsRootCause = true
		return &InterruptSignal{
			ID:            uuid.NewString(),
			Address:       addr,
			InterruptInfo: myPoint,
			InterruptState: InterruptState{
				State:                state,
				LayerSpecificPayload: config.LayerPayload,
			},
		}, nil
	}

	// 把子中断也一起返回
	return &InterruptSignal{
		ID:            uuid.NewString(),
		Address:       addr,
		InterruptInfo: myPoint,
		InterruptState: InterruptState{
			State:                state,
			LayerSpecificPayload: config.LayerPayload,
		},
		Subs: subContexts,
	}, nil
}

type InterruptCtx struct {
	ID          string
	Address     Address
	Info        any
	IsRootCause bool
	Parent      *InterruptCtx
}

func (ic *InterruptCtx) EqualsWithoutID(other *InterruptCtx) bool {
	if ic == nil && other == nil {
		return true
	}

	if ic == nil || other == nil {
		return false
	}

	if !ic.Address.Equals(other.Address) {
		return false
	}

	if ic.IsRootCause != other.IsRootCause {
		return false
	}

	if ic.Info != nil || other.Info != nil {
		if ic.Info == nil || other.Info == nil {
			return false
		}

		// reflect.DeepEqual 会做“深度比较”，比较的是内容，而不只是最外层引用
		if !reflect.DeepEqual(ic.Info, other.Info) {
			return false
		}
	}

	if ic.Parent != nil || other.Parent != nil {
		if ic.Parent == nil || other.Parent == nil {
			return false
		}

		// 递归往父母查
		if !ic.Parent.EqualsWithoutID(other.Parent) {
			return false
		}
	}

	return true
}

type InterruptContextsProvider interface {
	GetInterruptContexts() []*InterruptCtx
}

// 把数组变成树
func FromInterruptContexts(contexts []*InterruptCtx) *InterruptSignal {
	if len(contexts) == 0 {
		return nil
	}

	signalMap := make(map[string]*InterruptSignal)
	var rootSignal *InterruptSignal

	var getOrCreateSignal func(*InterruptCtx) *InterruptSignal
	getOrCreateSignal = func(ctx *InterruptCtx) *InterruptSignal {
		if ctx == nil {
			return nil
		}

		// 用 map 缓存，避免重复创建节点
		if signal, exists := signalMap[ctx.ID]; exists {
			return signal
		}

		newSignal := &InterruptSignal{
			ID:      ctx.ID,
			Address: ctx.Address,
			InterruptInfo: InterruptInfo{
				Info:        ctx.Info,
				IsRootCause: ctx.IsRootCause,
			},
		}
		signalMap[ctx.ID] = newSignal

		if parentSignal := getOrCreateSignal(ctx.Parent); parentSignal != nil {
			// parent 添加当前节点为子节点
			parentSignal.Subs = append(parentSignal.Subs, newSignal)
		} else {
			// parent 为空，说明当前节点是根节点
			rootSignal = newSignal
		}

		return newSignal
	}

	for _, ctx := range contexts {
		// 利用副作用组装，所以返回值不重要
		_ = getOrCreateSignal(ctx)
	}

	return rootSignal
}

func IoInterruptContexts(is *InterruptSignal, allowedSegmentTypes []AddressSegmentType) []*InterruptCtx {
	if is == nil {
		return nil
	}

	var rootCauseContexts []*InterruptCtx

	// 注意这里不能用短变量声明，Go 里短变量声明的作用域是从声明语句结束后才开始生效，不是从 := 左边名字出现的那一刻就生效
	// 这里递归引用自己会出现未声明的情况
	var buildContexts func(*InterruptSignal, *InterruptCtx)
	buildContexts = func(signal *InterruptSignal, parentCtx *InterruptCtx) {
		currentCtx := &InterruptCtx{
			ID:          signal.ID,
			Address:     signal.Address,
			Info:        signal.InterruptInfo.Info,
			IsRootCause: signal.InterruptInfo.IsRootCause,
			Parent:      parentCtx,
		}

		if currentCtx.IsRootCause {
			rootCauseContexts = append(rootCauseContexts, currentCtx)
		}

		for _, subSignal := range signal.Subs {
			buildContexts(subSignal, currentCtx)
		}
	}

	buildContexts(is, nil)

	// 根据 allowedSegmentTypes 进行过滤和裁切
	if len(allowedSegmentTypes) > 0 {
		allowedSet := make(map[AddressSegmentType]bool, len(allowedSegmentTypes))
		for _, t := range allowedSegmentTypes {
			allowedSet[t] = true
		}

		for _, ctx := range rootCauseContexts {
			filterParentChain(ctx, allowedSet)
			encapsulateContextAddress(ctx, allowedSet)
		}
	}

	return rootCauseContexts
}

// 沿着 ctx.Parent 这条链往上过滤
func filterParentChain(ctx *InterruptCtx, allowedSet map[AddressSegmentType]bool) {
	if ctx == nil {
		return
	}

	parent := ctx.Parent
	// for 循环，一直往上找 parent，直到遇到满足条件的 parent
	for parent != nil {
		// address 不为空，且最后一个地址段的类型在 allowedSet 中
		if len(parent.Address) > 0 && allowedSet[parent.Address[len(parent.Address)-1].Type] {
			break
		}
		parent = parent.Parent
	}

	// 找成了“离 ctx 最近的合法父节点”
	ctx.Parent = parent

	// 不光要保证 ctx.Parent 合法，还要保证 ctx.Parent.Parent 、再往上的祖先链也都已经跳过非法节点
	filterParentChain(parent, allowedSet)
}

// InterruptCtx 及其所有父节点的 Address ，只保留 allowedSet 里允许的 AddressSegmentType
func encapsulateContextAddress(ctx *InterruptCtx, allowedSet map[AddressSegmentType]bool) {
	for c := ctx; c != nil; c = c.Parent {
		newAddr := make(Address, 0, len(c.Address))
		for _, seg := range c.Address {
			if allowedSet[seg.Type] {
				newAddr = append(newAddr, seg)
			}
		}
		c.Address = newAddr
	}
}

// 把一棵 InterruptSignal 树拍平成两个以 ID 为 key 的 map，方便持久化到 checkpoint
func SignalToPersistenceMaps(is *InterruptSignal) (map[string]Address, map[string]InterruptState) {
	id2addr := make(map[string]Address)
	id2state := make(map[string]InterruptState)

	if is == nil {
		return id2addr, id2state
	}

	var traverse func(*InterruptSignal)
	// 从给定的节点 dfs 遍历所有节点，获取 id2addr 和 id2state 信息
	traverse = func(signal *InterruptSignal) {
		id2addr[signal.ID] = signal.Address
		id2state[signal.ID] = signal.InterruptState

		for _, sub := range signal.Subs {
			traverse(sub)
		}
	}

	traverse(is)
	return id2addr, id2state
}
