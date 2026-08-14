package model

import (
	"github.com/cloudwego/eino/internal/callbacks"
	"github.com/cloudwego/eino/schema"
)

type CallbackInput struct {
	Messages   []*schema.Message
	Tools      []*schema.ToolInfo
	ToolChoice *schema.ToolChoice
	Config     *Config
	Extra      map[string]any
}

type Config struct {
	Model       string
	MaxToken    int
	Temperature float32
	TopP        float32
	Stop        []string
}

type TokenUsage struct {
	PromptTokens            int
	PromptTokenDetails      PromptTokenDetails
	CompletionTokens        int
	TotalTokens             int
	CompletionTokensDetails CompletionTokensDetails `json:"completion_token_details"`
}

type PromptTokenDetails struct {
	CachedTokens int
}

type CompletionTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

type CallbackOutput struct {
	Message    *schema.Message
	Config     *Config
	TokenUsage *TokenUsage
	Extra      map[string]any
}

func ConvCallbackInput(src callbacks.CallbackInput) *CallbackInput {
	switch t := src.(type) {
	case *CallbackInput:
		return t
	case []*schema.Message:
		return &CallbackInput{
			Messages: t,
		}
	default:
		return nil
	}
}

func ConvCallbackOutput(src callbacks.CallbackOutput) *CallbackOutput {
	switch t := src.(type) {
	case *CallbackOutput: // when callback is triggered within component implementation, the output is usually already a typed *model.CallbackOutput
		return t
	case *schema.Message: // when callback is injected by graph node, not the component implementation itself, the output is the output of Chat Model interface, which is *schema.Message
		return &CallbackOutput{
			Message: t,
		}
	default:
		return nil
	}
}
