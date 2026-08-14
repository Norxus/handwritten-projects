package adk

import "github.com/cloudwego/eino/callbacks"

type AgentCallBackInput struct {
	Input      *AgentInput
	ResumeInfo *ResumeInfo
}

type AgentInput struct {
	Messages        []Message
	EnableStreaming bool
}

type AgentCallbackOutput struct {
	Events *AsyncIterator[*AgentEvent]
}

func ConvAgentCallbackInput(input callbacks.CallbackInput) *AgentCallBackInput {
	if v, ok := input.(*AgentCallBackInput); ok {
		return v
	}
	return nil
}

func ConvAgentCallbackOutput(output callbacks.CallbackOutput) *AgentCallbackOutput {
	if v, ok := output.(*AgentCallbackOutput); ok {
		return v
	}
	return nil
}
