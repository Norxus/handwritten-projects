package prompt

import (
	"context"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/schema"
)

type DefaultChatTemplate struct {
	templates  []schema.MessagesTemplate
	formatType schema.FormatType
}

func (t *DefaultChatTemplate) Format(ctx context.Context, vs map[string]any, _ ...Option) (result []*schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, t.GetType(), components.ComponentOfPrompt)
	// 触发开始执行的回调
	ctx = callbacks.OnStart(ctx, &CallbackInput{
		Variables: vs,
		Templates: t.templates,
	})
	// 触发失败的回调
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	// 每个模板都渲染，然后把渲染依次拼接
	result = make([]*schema.Message, 0, len(t.templates))
	for _, template := range t.templates {
		msgs, err := template.Format(ctx, vs, t.formatType)
		if err != nil {
			return nil, err
		}

		result = append(result, msgs...)
	}

	// 成功之后调用 OnEnd
	_ = callbacks.OnEnd(ctx, &CallbackOutput{
		Result:   result,
		Template: t.templates,
	})

	return result, nil
}

func (t *DefaultChatTemplate) GetType() string {
	return "Default"
}

func (t *DefaultChatTemplate) IsCallbacksEnabled() bool {
	return true
}
