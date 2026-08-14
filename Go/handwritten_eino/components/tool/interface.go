package tool

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

type BaseTool interface {
	Info(ctx context.Context) (*schema.ToolInfo, error)
}

type InvokableTool interface {
	BaseTool

	InvokableRun(ctx context.Context, argumentsInJSON string, opts ...Option) (string, error)
}

type StreamableTool interface {
	BaseTool

	StreamableRun(ctx context.Context, argumentsInJSON string, opts ...Option) (*schema.StreamReader[string], error)
}

type EnhancedInvokableTool interface {
	BaseTool

	InvokableRun(ctx context.Context, toolArgument *schema.ToolArgument, opts ...Option) (*schema.ToolResult, error)
}

type EnhancedStreamableTool interface {
	BaseTool

	StreamableRun(ctx context.Context, toolArgument *schema.ToolArgument, opts ...Option) (*schema.StreamReader[*schema.ToolResult], error)
}
