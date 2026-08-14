package compose

import "reflect"

type graphAddNodeOpts struct {
	nodeOptions *nodeOptions
	processor   *processorOpts

	needState bool
}

type GraphAddNodeOpt func(o *graphAddNodeOpts)

type nodeOptions struct {
	nodeName string

	nodeKey string

	inputKey  string
	outputKey string

	graphCompileOption []GraphCompileOption
}

type processorOpts struct {
	statePreHandler  *composableRunnable
	preStateType     reflect.Type
	statePostHandler *composableRunnable
	postStateType    reflect.Type
}

func getGraphAddNodeOpts(opts ...GraphAddNodeOpt) *graphAddNodeOpts {
	opt := &graphAddNodeOpts{
		nodeOptions: &nodeOptions{
			nodeName: "",
			nodeKey:  "",
		},
		processor: &processorOpts{
			statePreHandler:  nil,
			statePostHandler: nil,
		},
	}

	for _, fn := range opts {
		fn(opt)
	}

	return opt
}
