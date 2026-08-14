package prompt

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

var _ ChatTemplate = &DefaultChatTemplate{}

type ChatTemplate interface {
	Format(ctx context.Context, vs map[string]any, opts ...Option) ([]*schema.Message, error)
}

func FromMessage(formatType schema.FormatType, templates ...schema.MessagesTemplate) *DefaultChatTemplate {
	return &DefaultChatTemplate{
		templates:  templates,
		formatType: formatType,
	}
}
