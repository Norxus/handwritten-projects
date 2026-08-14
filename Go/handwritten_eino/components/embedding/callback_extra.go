package embedding

import "github.com/cloudwego/eino/internal/callbacks"

type CallbackInput struct {
	Texts  []string
	Config *Config
	Extra  map[string]any
}

type Config struct {
	Model          string
	EncodingFormat string
}

type CallbackOutput struct {
	Embeddings [][]float64
	Config     *Config
	TokenUsage *TokenUsage
	Extra      map[string]any
}

type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

func ConvCallbackInput(src callbacks.CallbackInput) *CallbackInput {
	switch t := src.(type) {
	case *CallbackInput:
		return t
	case []string:
		return &CallbackInput{
			Texts: t,
		}
	default:
		return nil
	}
}

func ConvCallbackOutput(src callbacks.CallbackOutput) *CallbackOutput {
	switch t := src.(type) {
	case *CallbackOutput:
		return t
	case [][]float64:
		return &CallbackOutput{
			Embeddings: t,
		}
	default:
		return nil
	}
}
