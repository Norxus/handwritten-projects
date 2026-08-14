package model

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

type BaseChatModel interface {
	Generate(ctx context.Context, input []*schema.Message, opts ...Option) (*schema.Message, error)
	Stream(ctx context.Context, input []*schema.Message, opts ...Option) (*schema.StreamReader[*schema.Message], error)
}

type ChatModel interface {
	BaseChatModel

	BindTools(tools []*schema.ToolInfo) error
}

type ToolCallingChatModel interface {
	BaseChatModel

	WithTools(tools []*schema.ToolInfo) (ToolCallingChatModel, error)
}
