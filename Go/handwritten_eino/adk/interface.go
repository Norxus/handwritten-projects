package adk

import (
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/internal/core"
	"github.com/cloudwego/eino/schema"
)

const ComponentOfAgent components.Component = "Agent"

type Message = *schema.Message

type AgentEvent struct {
	AgentName string

	RunPath []RunStep
	Output  *AgentOutput
	Action  *AgentAction
	Err     error
}

type RunStep struct {
	agentName string
}

type AgentOutput struct {
	MessageOutput    *MessageVariant
	CustomizedOutput any
}

type MessageVariant struct {
	IsStreaming   bool
	Message       Message
	MessageStream MessageStream
	Role          schema.RoleType
	ToolName      string
}

type MessageStream = *schema.StreamReader[Message]

type AgentAction struct {
	Exit bool

	Interrupted *InterruptInfo

	TransferToAgent *TransferToAgentAction

	BreakLoop *BreakLoopAction

	CustomizedAction any

	InternalInterrupted *core.InterruptSignal
}

type TransferToAgentAction struct {
	DestAgentName string
}

type BreakLoopAction struct {
	From              string
	Done              bool
	CurrentIterations int
}
