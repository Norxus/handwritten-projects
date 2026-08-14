package tool

import (
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
)

type CallbackInput struct {
	ArgumentInJSON string
	Extra          map[string]any
}

type CallbackOutput struct {
	Response   string
	ToolOutput *schema.ToolResult
	Extra      map[string]any
}

func ConvCallbackInput(src callbacks.CallbackInput) *CallbackInput {
	switch t := src.(type) {
	case *CallbackInput:
		return t
	case string:
		return &CallbackInput{ArgumentInJSON: t}
	case *schema.ToolArgument:
		return &CallbackInput{
			ArgumentInJSON: t.Text,
		}
	default:
		return nil
	}
}

func ConvCallbackOutput(src callbacks.CallbackOutput) *CallbackOutput {
	switch t := src.(type) {
	case *CallbackOutput:
		return t
	case string:
		return &CallbackOutput{Response: t}
	case *schema.ToolResult:
		return &CallbackOutput{ToolOutput: t}
	default:
		return nil
	}
}
