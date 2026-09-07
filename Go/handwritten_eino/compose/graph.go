package compose

import (
	"context"
	"reflect"
)

type graph struct {
	// 节点表 node_key -> 实际 node
	nodes map[string]*graphNode
	// 控制边，表示执行依赖关系，起点节点 key -> 终点节点 key 列表
	controlEdges map[string][]string
	// 数据边，表示数据流向。上游输出会传给下游输入，起点节点 key -> 数据流向的终点节点 key 列表
	dataEdges map[string][]string
	// 分支边，某个节点执行后通过 branch condition 动态选择后继节点，branch 挂在哪个起点节点后面 -> 该起点节点上的分支列表
	branches map[string][]*GraphBranch
	// 所有从虚拟节点 START 直接连出去的节点
	startNodes []string
	// 所有直接连到虚拟节点 END 的节点
	endNodes []string

	// 还没完成类型校验的数据边待办表，起始节点 -> 从这个起点出发、等待校验的一组终点信息
	// 有些节点的输入/输出类型在添加边时还不能立刻确定，
	// 尤其是 passthrough 节点，passthrough 本身没有固定类型，它的类型要根据前后节点推断出来。所以添加边时不能马上判断
	// 对于这种情况就先放到 toValidateMap，然后 updateToValidateMap() 会反复尝试
	toValidateMap map[string][]struct {
		// 边上的终点节点
		endNode string
		// 这条边上配置的字段映射规则，可能为空
		mappings []*FieldMapping
	}

	// state 的类型，所谓 state 就是图运行时各个节点的一个共享变量
	stateType      reflect.Type
	// 每次运行 graph 时创建一份本地 state 的函数。
	stateGenerator func(ctx context.Context) any
	newOpts        []NewGraphOption

	// 图的输入输出类型
	expectedInputType, expectedOutputType reflect.Type

	*genericHelper

	fieldMappingRecord map[string][]*FieldMapping

	buildError error

	cmp component

	compiled bool

	handlerOnEdges   map[string]map[string][]handlerPair
	handlerPreNode   map[string][]handlerPair
	handlerPreBranch map[string][][]handlerPair
}
