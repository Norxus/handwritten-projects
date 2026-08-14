package compose

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

type nodeOptionsPair generic.Pair[*graphNode, *graphAddNodeOpts]

type ChainBranch struct {
	internalBranch *GraphBranch
	key2BranchNode map[string]nodeOptionsPair
	err            error
}

func (cb *ChainBranch) addNode(key string, node *graphNode, options *graphAddNodeOpts) *ChainBranch {
	// 如果分支已经处于错误状态，就直接返回
	if cb.err != nil {
		return cb
	}

	// 确保内部 map 已初始化
	if cb.key2BranchNode == nil {
		cb.key2BranchNode = make(map[string]nodeOptionsPair)
	}

	// 不能重复添加 node
	_, ok := cb.key2BranchNode[key]
	if ok {
		cb.err = fmt.Errorf("chain branch add node, duplicate branch node key= %s", key)
		return cb
	}

	// 保存节点信息和 options 信息
	cb.key2BranchNode[key] = nodeOptionsPair{node, options}

	return cb
}

func NewChainMultiBranch[T any](cond GraphMultiBranchCondition[T]) *ChainBranch {
	// 根据输入判断要走哪些节点
	invokeCond := func(ctx context.Context, in T, opts ...any) (endNodes []string, err error) {
		ends, err := cond(ctx, in)
		if err != nil {
			return nil, err
		}
		endNodes = make([]string, 0, len(ends))
		for end := range ends {
			endNodes = append(endNodes, end)
		}
		return endNodes, nil
	}

	return &ChainBranch{
		key2BranchNode: make(map[string]nodeOptionsPair),
		internalBranch: newGraphBranch(newRunnablePacker(invokeCond, nil, nil, nil, false), nil),
	}
}

func NewStreamChainMultiBranch[T any](cond StreamGraphMultiBranchCondition[T]) *ChainBranch {
	collectCon := func(ctx context.Context, in *schema.StreamReader[T], opts ...any) (endNodes []string, err error) {
		ends, err := cond(ctx, in)
		if err != nil {
			return nil, err
		}

		endNodes = make([]string, 0, len(ends))
		for end := range ends {
			endNodes = append(endNodes, end)
		}

		return endNodes, nil
	}

	return &ChainBranch{
		key2BranchNode: make(map[string]nodeOptionsPair),
		internalBranch: newGraphBranch(newRunnablePacker(nil, nil, collectCon, nil, false), nil),
	}
}

func NewChainBranch[T any](cond GraphBranchCondition[T]) *ChainBranch {
	return NewChainMultiBranch(func(ctx context.Context, in T) (endNode map[string]bool, err error) {
		ret, err := cond(ctx, in)
		if err != nil {
			return nil, err
		}
		return map[string]bool{ret: true}, nil
	})
}

func NewStreamChainBranch[T any](cond StreamGraphBranchCondition[T]) *ChainBranch {
	return NewStreamChainMultiBranch(func(ctx context.Context, in *schema.StreamReader[T]) (endNodes map[string]bool, err error) {
		ret, err := cond(ctx, in)
		if err != nil {
			return nil, err
		}
		return map[string]bool{ret: true}, nil
	})
}

func (cb *ChainBranch) AddChatModel(key string, node model.BaseChatModel, opts ...GraphAddNodeOpt) *ChainBranch {
	gNode, options := toChatModelNode(node, opts...)
	return cb.addNode(key, gNode, options)
}

func (cb *ChainBranch) AddChatTemplate(key string, node prompt.ChatTemplate, opts ...GraphAddNodeOpt) *ChainBranch {
	gNode, options := toChatTemplateNode(node, opts...)
	return cb.addNode(key, gNode, options)
}

func (cb *ChainBranch) AddToolsNode(key string, node *ToolsNode, opts ...GraphAddNodeOpt) *ChainBranch {
	gNode, options := toToolsNode(node, opts...)
	return cb.addNode(key, gNode, options)
}

