package compose

import (
	"reflect"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/internal/generic"
)

type executorMeta struct {
	component component

	isComponentCallbackEnabled bool

	componentImplType string
}

type nodeInfo struct {
	name string

	inputKey  string
	outputKey string

	preProcessor, postProcessor *composableRunnable

	compileOption *graphCompileOptions
}

type graphNode struct {
	cr *composableRunnable
	g  AnyGraph

	nodeInfo     *nodeInfo
	executorMeta *executorMeta

	instance any
	opts     []GraphAddNodeOpt
}

// 获取组件的类型名，组件是否支持 callback
func parseExecutorInfoFromComponent(c component, executor any) *executorMeta {
	componentImplType, ok := components.GetType(executor)
	if !ok {
		componentImplType = generic.ParseTypeName(reflect.ValueOf(executor))
	}

	return &executorMeta{
		component:                  c,
		isComponentCallbackEnabled: components.IsCallbacksEnabled(executor),
		componentImplType:          componentImplType,
	}
}

// 把一堆可变参数归并成一个总配置 opt，再从 opt 里挑出建 graphNode 最核心的字段，组装成 nodeInfo
func getNodeInfo(opts ...GraphAddNodeOpt) (*nodeInfo, *graphAddNodeOpts) {
	opt := getGraphAddNodeOpts(opts...)

	return &nodeInfo{
		name:          opt.nodeOptions.nodeName,
		inputKey:      opt.nodeOptions.inputKey,
		outputKey:     opt.nodeOptions.outputKey,
		preProcessor:  opt.processor.statePreHandler,
		postProcessor: opt.processor.statePostHandler,
		compileOption: newGraphCompileOptions(opt.nodeOptions.graphCompileOption...),
	}, opt
}
