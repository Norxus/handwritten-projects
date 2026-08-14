package document

import (
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
)

type TransformerCallbackInput struct {
	Input []*schema.Document
	Extra map[string]any
}

type TransformerCallbackOutput struct {
	Output []*schema.Document
	Extra  map[string]any
}

func ConvTransformerCallbackInput(src callbacks.CallbackInput) *TransformerCallbackInput {
	switch t := src.(type) {
	case *TransformerCallbackInput:
		return t
	case []*schema.Document:
		return &TransformerCallbackInput{
			Input: t,
		}
	default:
		return nil
	}
}

func ConvTransformerCallbackOutput(src callbacks.CallbackOutput) *TransformerCallbackOutput {
	switch t := src.(type) {
	case *TransformerCallbackOutput:
		return t
	case []*schema.Document:
		return &TransformerCallbackOutput{
			Output: t,
		}
	default:
		return nil
	}
}
