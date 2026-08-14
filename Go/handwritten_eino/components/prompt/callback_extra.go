package prompt

import (
	"github.com/cloudwego/eino/internal/callbacks"
	"github.com/cloudwego/eino/schema"
)

type CallbackInput struct {
	Variables map[string]any
	Templates []schema.MessagesTemplate
	Extra     map[string]any
}

type CallbackOutput struct {
	Result   []*schema.Message
	Template []schema.MessagesTemplate
	Extra    map[string]any
}

// 兼容转换工具，CallbackInput 实际上就是 any 类型，所以可以传入 map[string]any
func ConvCallbackInput(src callbacks.CallbackInput) *CallbackInput {
	switch t := src.(type) {
	case *CallbackInput:
		return t
	case map[string]any:
		return &CallbackInput{
			Variables: t,
		}
	default:
		return nil
	}
}

func ConvCallbackOutput(src callbacks.CallbackOutput) *CallbackOutput {
	switch t := src.(type) {
	case *CallbackOutput:
		return t
	case []*schema.Message:
		return &CallbackOutput{
			Result: t,
		}
	default:
		return nil
	}
}
