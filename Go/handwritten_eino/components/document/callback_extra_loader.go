package document

import (
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
)

type LoaderCallbackInput struct {
	Source Source
	Extra  map[string]any
}

type LoaderCallbackOutput struct {
	Source Source
	Docs   []*schema.Document
	Extra  map[string]any
}

func ConvLoaderCallbackInput(src callbacks.CallbackInput) *LoaderCallbackInput {
	switch t := src.(type) {
	case *LoaderCallbackInput:
		return t
	case Source:
		return &LoaderCallbackInput{
			Source: t,
		}
	default:
		return nil
	}
}

func ConvLoaderCallbackOutput(src callbacks.CallbackOutput) *LoaderCallbackOutput {
	switch t := src.(type) {
	case *LoaderCallbackOutput:
		return t
	case []*schema.Document:
		return &LoaderCallbackOutput{
			Docs: t,
		}
	default:
		return nil
	}
}
