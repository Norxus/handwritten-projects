package callbacks

import (
	"context"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/schema"
)

type RunInfo struct {
	Name      string
	Type      string
	Component components.Component
}

type CallbackInput any

type CallbackOutput any

type CallbackTiming uint8

type Handler interface {
	OnStart(ctx context.Context, info *RunInfo, input CallbackInput) context.Context
	OnEnd(ctx context.Context, info *RunInfo, ouput CallbackOutput) context.Context

	OnError(ctx context.Context, info *RunInfo, err error) context.Context

	OnStartWithStreamInput(ctx context.Context, info *RunInfo,
		input *schema.StreamReader[CallbackInput]) context.Context

	OnEndWithStreamOutput(ctx context.Context, info *RunInfo,
		output *schema.StreamReader[CallbackOutput]) context.Context
}

type TimingChecker interface {
	Needed(ctx context.Context, info *RunInfo, timing CallbackTiming) bool
}
