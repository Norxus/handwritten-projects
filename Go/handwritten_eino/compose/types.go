package compose

import "github.com/cloudwego/eino/components"

type component = components.Component

const (
	ComponentOfUnknown     component = "Unknown"
	ComponentOfGraph       component = "Graph"
	ComponentOfWorkflow    component = "Workflow"
	ComponentOfChain       component = "Chain"
	ComponentOfPassthrough component = "Passthrough"
	ComponentOfToolsNode   component = "ToolsNode"
	ComponentOfLambda      component = "Lambda"
)

type NodeTriggerMode string

const (
	AnyPredecessor NodeTriggerMode = "any_predecessor"
	AllPredecessor NodeTriggerMode = "all_predecessor"
)
