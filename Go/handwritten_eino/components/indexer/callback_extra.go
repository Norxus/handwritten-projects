package indexer

import (
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
)

type CallbackInput struct {
	Docs  []*schema.Document
	Extra map[string]any
}

type CallbackOutput struct {
	IDs   []string
	Extra map[string]any
}

func ConvCallbackInput(src callbacks.CallbackInput) *CallbackInput {
	switch t := src.(type) {
	case *CallbackInput:
		return t
	case []*schema.Document:
		return &CallbackInput{
			Docs: t,
		}
	default:
		return nil
	}
}

func ConvCallbackOutput(src callbacks.CallbackOutput) *CallbackOutput {
	switch t := src.(type) {
	case *CallbackOutput:
		return t
	case []string:
		return &CallbackOutput{
			IDs: t,
		}
	default:
		return nil
	}
}
