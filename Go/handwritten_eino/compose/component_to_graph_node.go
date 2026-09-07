package compose

import (
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
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

// nodeInfo: 节点的图内信息，比如节点名、分支等，后续图编排、边连接、错误定位、callback 地址等都依赖它识别“这是哪个节点”
// executor: 里面是 invoke, transform 等函数
// meta: 执行器的元信息，主要描述这个节点属于什么组件、callback 是否已启用、实现类型是什么等
// instance: 原始节点实例，因为 executor 会包装原始实例的方法，但是丢失原始实例的信息，便于后续其他操作和观测
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

func toLambdaNode(node *Lambda, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	info, options := getNodeInfo(opts...)
	gn := toNode(info, node.executor, nil, node.executor.meta, node, opts...)
	return gn, options
}

func toAnyGraphNode(node AnyGraph, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	meta := parseExecutorInfoFromComponent(node.component(), node)
	info, options := getNodeInfo(opts...)
	gn := toNode(info, nil, node, meta, node, opts...)
	return gn, options
}

func toPassthroughNode(opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	node := composablePassthrough()
	info, options := getNodeInfo(opts...)
	gn := toNode(info, node, nil, node.meta, node, opts...)
	return gn, options
}

func toDocumentTransformerNode(node document.Transformer, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		components.ComponentOfTransformer,
		node.Transform,
		nil,
		nil,
		nil,
		opts...)
}

func toEmbeddingNode(node embedding.Embedder, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		components.ComponentOfEmbedding,
		node.EmbedStrings,
		nil,
		nil,
		nil,
		opts...)
}

func toRetrieverNode(node retriever.Retriever, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		components.ComponentOfRetriever,
		node.Retrieve,
		nil,
		nil,
		nil,
		opts...)
}

func toLoaderNode(node document.Loader, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		components.ComponentOfLoader,
		node.Load,
		nil,
		nil,
		nil,
		opts...)
}

func toIndexerNode(node indexer.Indexer, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		components.ComponentOfIndexer,
		node.Store,
		nil,
		nil,
		nil,
		opts...)
}
