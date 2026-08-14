package retriever

import (
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
)

type CallbackInput struct {
	Query          string
	TopK           int
	Filter         string
	ScoreThreshold *float64
	Extra          map[string]any
}

type CallbackOutput struct {
	Docs  []*schema.Document
	Extra map[string]any
}

func ConvCallbackInput(src callbacks.CallbackInput) *CallbackInput {
	switch t := src.(type) {
	case *CallbackInput:
		return t
	case string:
		return &CallbackInput{
			Query: t,
		}
	default:
		return nil
	}
}

func ConvCallbackOutput(src callbacks.CallbackOutput) *CallbackOutput {
	switch t := src.(type) {
	case *CallbackOutput:
		return t
	case []*schema.Document:
		return &CallbackOutput{
			Docs: t,
		}
	default:
		return nil
	}
}
