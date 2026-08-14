package compose

import (
	"context"
	"reflect"

	"github.com/cloudwego/eino/components"
)

type GraphNodeInfo struct {
	Component             components.Component
	Instance              any
	GraphAddNodeOpts      []GraphAddNodeOpt
	InputType, OutputType reflect.Type
	Name                  string
	InputKey, OutputKey   string
	GraphInfo             *GraphInfo
	Mappings              []*FieldMapping
}

type GraphInfo struct {
	CompileOptions        []GraphCompileOption
	Nodes                 map[string]GraphNodeInfo
	Edges                 map[string][]string
	DataEdges             map[string][]string
	Branches              map[string][]GraphBranch
	InputType, OutputType reflect.Type
	Name                  string
	NewGraphOptions       []NewGraphOption
	GenStateFn            func(context.Context) any
}

type GraphCompileCallback interface {
	OnFinish(ctx context.Context, info *GraphInfo)
}
