package compose

import (
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
)

func toComponentNode[I, O, TOption any](
	node any,
	componentType component,
	invoke Invoke[I, O, TOption],
	stream Stream[I, O, TOption],
	collect Collect[I, O, TOption],
	transform Transform[I, O, TOption],
	opts ...GraphAddNodeOpt,
) (*graphNode, *graphAddNodeOpts) {
	// 传入组件类型和实际的 node
	meta := parseExecutorInfoFromComponent(componentType, node)
	info, options := getNodeInfo(opts...)
	// 最后一个参数标识要不要由框架帮这个组件包 callback 逻辑
	run := runnableLambda(invoke, stream, collect, transform,
		!meta.isComponentCallbackEnabled,
	)

	// 把上面的参数组装成一个节点
	gn := toNode(info, run, nil, meta, node, opts...)

	return gn, options
}

// 组装 ChatModel 节点
func toChatModelNode(node model.BaseChatModel, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		components.ComponentOfChatModel,
		node.Generate,
		node.Stream,
		nil,
		nil,
		opts...,
	)
}

func toChatTemplateNode(node prompt.ChatTemplate, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		components.ComponentOfPrompt,
		node.Format,
		nil,
		nil,
		nil,
		opts...,
	)
}

func toToolsNode(node *ToolsNode, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		ComponentOfToolsNode,
		node.Invoke,
		node.Stream,
		nil,
		nil,
		opts...,
	)
}

func toNode(nodeInfo *nodeInfo, executor *composableRunnable, graph AnyGraph,
	meta *executorMeta, instance any, opts ...GraphAddNodeOpt) *graphNode {

	if meta == nil {
		meta = &executorMeta{}
	}

	gn := &graphNode{
		nodeInfo:     nodeInfo,
		cr:           executor,
		executorMeta: meta,
		instance:     instance,
		opts:         opts,
	}

	return gn
}
