package compose

import (
	"context"
	"errors"

	"github.com/cloudwego/eino/internal/core"
	"github.com/cloudwego/eino/schema"
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
	InterruptContext []*InterruptCtx
}

type AddressSegmentType = core.AddressSegmentType

const (
	AddressSegmentNode     AddressSegmentType = "node"
	AddressSegmentTool     AddressSegmentType = "tool"
	AddressSegmentRunnable AddressSegmentType = "runnable"
)

func init() {
	schema.RegisterName[*InterruptInfo]("_eino_compose_interrupt_info")
}

func WrapInterruptAndRerunIfNeeded(ctx context.Context, step AddressSegment, err error) error {
	
}
