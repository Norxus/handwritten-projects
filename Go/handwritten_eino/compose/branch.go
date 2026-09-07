package compose

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cloudwego/eino/internal/generic"

	"github.com/cloudwego/eino/schema"
)

type GraphBranchCondition[T any] func(ctx context.Context, in T) (endNode string, err error)

type StreamGraphBranchCondition[T any] func(ctx context.Context, in *schema.StreamReader[T]) (endNode string, err error)

// 图的多分支判断函数
type GraphMultiBranchCondition[T any] func(ctx context.Context, in T) (endNode map[string]bool, err error)

type StreamGraphMultiBranchCondition[T any] func(ctx context.Context, in *schema.StreamReader[T]) (endNodes map[string]bool, err error)

// 图调度中的分支结构，用来在某个节点执行完之后，根据输入动态决定接下来走到哪些节点
type GraphBranch struct {
	// 用于处理普通输入，输出选择出来的后继节点
	invoke func(ctx context.Context, input any) (output []string, err error)
	// 用于处理流式输入，输出选择出来的后继节点
	collect func(ctx context.Context, input streamReader) (output []string, err error)
	// inputType 和 genericHelper 让图编译、连接、类型检查时知道这个 branch 需要什么输入类型
	inputType reflect.Type
	*genericHelper
	// 记录这个分支允许跳转到哪些节点
	endNodes map[string]bool
	// 并行执行分支时的编号
	idx int
	// 表明是否该分支只控制路由，不参与普通数据流传递，也就是不会讲上一节点的输出作为输入传入下一个节点
	noDataFlow bool
}

func (gb *GraphBranch) GetEndNode() map[string]bool {
	return gb.endNodes
}

// 把一个泛型的 runnablePacker[T, []string, any] 包装成统一的 *GraphBranch
func newGraphBranch[T any](r *runnablePacker[T, []string, any], endNodes map[string]bool) *GraphBranch {
	return &GraphBranch{
		invoke: func(ctx context.Context, input any) (output []string, err error) {
			in, ok := input.(T)
			if !ok {
				// 如果传进来的 input 就是裸 nil，并且目标类型 T 是“接口类型”，就创建一个类型 T 的 nil 值
				// 因为 nil 断言根本没办法知道它的具体类型
				// 同时只有接口类型比较适合接这种“typed nil”场景
				// 如果 T 是具体类型，比如 string 、 int 、结构体，那裸 nil 本来就不是一个合法的 T
				// 这时就应该报错/ panic ，而不是偷偷兜底。
				if input == nil && generic.TypeOf[T]().Kind() == reflect.Interface {
					var i T
					in = i
				} else {
					panic(newUnexpectedInputTypeErr(generic.TypeOf[T](), reflect.TypeOf(input)))
				}
			}
			return r.Invoke(ctx, in)
		},
		collect: func(ctx context.Context, input streamReader) (output []string, err error) {
			in, ok := unpackStreamReader[T](input)
			if !ok {
				panic(newUnexpectedInputTypeErr(generic.TypeOf[T](), input.getType()))
			}
			return r.Collect(ctx, in)
		},
		inputType:     generic.TypeOf[T](),
		genericHelper: newGenericHelper[T, T](),
		endNodes:      endNodes,
	}
}

// 把你传入的分支判断函数 condition ，包装成一个 GraphBranch ，并且校验返回的分支节点是否都在允许范围里
func NewGraphMultiBranch[T any](conditon GraphMultiBranchCondition[T], endNodes map[string]bool) *GraphBranch {
	// 根据给定的输入 in 决定要走哪些节点，返回接下来要走哪些节点
	condRun := func(ctx context.Context, in T, opts ...any) ([]string, error) {
		// 判断要走哪些 end 节点
		ends, err := conditon(ctx, in)
		if err != nil {
			return nil, err
		}
		ret := make([]string, 0, len(ends))
		for end := range ends {
			// 检查这个节点是不是在预先声明的合法节点集合 endNodes 里
			if !endNodes[end] {
				return nil, fmt.Errorf("branch invocation returns unintended end node: %s", end)
			}
			ret = append(ret, end)
		}
		return ret, nil
	}
	// codeRun 放入 invoke 中
	return newGraphBranch(newRunnablePacker(condRun, nil, nil, nil, false), endNodes)
}

// 创建一个支持多目标节点的分支，condition 函数处理流式输入
func NewStreamGraphMultiBranch[T any](condition StreamGraphMultiBranchCondition[T],
	endNodes map[string]bool) *GraphBranch {
	codeRun := func(ctx context.Context, in *schema.StreamReader[T], opts ...any) ([]string, error) {
		ends, err := condition(ctx, in)
		if err != nil {
			return nil, err
		}

		ret := make([]string, 0, len(ends))
		for end := range ends {
			if !endNodes[end] {
				return nil, fmt.Errorf("branch invocation returns unintended end node: %s", end)
			}
			ret = append(ret, end)
		}

		return ret, nil
	}

	// codeRun 放入 collect 中
	return newGraphBranch(newRunnablePacker(nil, nil, codeRun, nil, false), endNodes)
}

// 复用了 NewGraphMultiBranch，但是实际上只选择一个后继节点
func NewGraphBranch[T any](condition GraphBranchCondition[T], endNodes map[string]bool) *GraphBranch {
	return NewGraphMultiBranch(func(ctx context.Context, in T) (endNodes map[string]bool, err error) {
		ret, err := condition(ctx, in)
		if err != nil {
			return nil, err
		}
		return map[string]bool{ret: true}, nil
	}, endNodes)
}

// 同样的单个节点模式
func NewStreamGraphBranch[T any](condition StreamGraphBranchCondition[T], endNodes map[string]bool) *GraphBranch {
	return NewStreamGraphMultiBranch(func(ctx context.Context, in *schema.StreamReader[T]) (endNode map[string]bool, err error) {
		ret, err := condition(ctx, in)
		if err != nil {
			return nil, err
		}
		return map[string]bool{ret: true}, nil
	}, endNodes)
}
