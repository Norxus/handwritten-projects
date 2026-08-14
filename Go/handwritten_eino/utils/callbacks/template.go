package callbacks

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func NewHandlerHelper() *HandlerHelper {
	return &HandlerHelper{
		composeTemplates: map[components.Component]callbacks.Handler{},
	}
}

type HandlerHelper struct {
	promptHandler      *PromptCallbackHandler
	chatModelHandler   *ModelCallbackHandler
	embeddingHandler   *EmbeddingCallbackHandler
	indexerHandler     *IndexerCallbackHandler
	retrieverHandler   *RetrieverCallbackHandler
	loaderHandler      *LoaderCallbackHandler
	transformerHandler *TransformerCallbackHandler
	toolHandler        *ToolCallbackHandler
	toolsNodeHandler   *ToolsNodeCallbackHandlers
	agentHandler       *AgentCallbackHandler
	composeTemplates   map[components.Component]callbacks.Handler
}

type PromptCallbackHandler struct {
	OnStart func(ctx context.Context, runInfo *callbacks.RunInfo, input *prompt.CallbackInput) context.Context

	OnEnd func(ctx context.Context, runInfo *callbacks.RunInfo, output *prompt.CallbackOutput) context.Context

	OnError func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context
}

func (ch *PromptCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

type ModelCallbackHandler struct {
	OnStart func(ctx context.Context, runInfo *callbacks.RunInfo, input *model.CallbackInput) context.Context

	OnEnd func(ctx context.Context, runInfo *callbacks.RunInfo, output *model.CallbackOutput) context.Context

	OnEndWithStreamOutput func(ctx context.Context, runInfo *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context

	OnError func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context
}

func (ch *ModelCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	case callbacks.TimingOnEndWithStreamOutput:
		return ch.OnEndWithStreamOutput != nil
	default:
		return false
	}
}

type EmbeddingCallbackHandler struct {
	OnStart func(ctx context.Context, runInfo *callbacks.RunInfo, input *embedding.CallbackInput) context.Context

	OnEnd func(ctx context.Context, runInfo *callbacks.RunInfo, output *embedding.CallbackOutput) context.Context

	OnError func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context
}

type IndexerCallbackHandler struct {
	OnStart func(ctx context.Context, runInfo *callbacks.RunInfo, input *indexer.CallbackInput) context.Context
	OnEnd   func(ctx context.Context, runInfo *callbacks.RunInfo, output *indexer.CallbackOutput) context.Context
	OnError func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context
}

func (ch *EmbeddingCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

type RetrieverCallbackHandler struct {
	OnStart func(ctx context.Context, runInfo *callbacks.RunInfo, input *retriever.CallbackInput) context.Context
	OnEnd   func(ctx context.Context, runInfo *callbacks.RunInfo, output *retriever.CallbackOutput) context.Context
	OnError func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context
}

func (ch *IndexerCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

func (ch *RetrieverCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

type LoaderCallbackHandler struct {
	OnStart func(ctx context.Context, runInfo *callbacks.RunInfo, input *document.LoaderCallbackInput) context.Context
	OnEnd   func(ctx context.Context, runInfo *callbacks.RunInfo, output *document.LoaderCallbackOutput) context.Context
	OnError func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context
}

func (ch *LoaderCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

type TransformerCallbackHandler struct {
	OnStart func(ctx context.Context, runInfo *callbacks.RunInfo, input *document.TransformerCallbackInput) context.Context
	OnEnd   func(ctx context.Context, runInfo *callbacks.RunInfo, output *document.TransformerCallbackOutput) context.Context
	OnError func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context
}

func (ch *TransformerCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

type ToolCallbackHandler struct {
	OnStart               func(ctx context.Context, info *callbacks.RunInfo, input *tool.CallbackInput) context.Context
	OnEnd                 func(ctx context.Context, info *callbacks.RunInfo, output *tool.CallbackOutput) context.Context
	OnEndWithStreamOutput func(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[*tool.CallbackOutput]) context.Context
	OnError               func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context
}

func (ch *ToolCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnEndWithStreamOutput:
		return ch.OnEndWithStreamOutput != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

type ToolsNodeCallbackHandlers struct {
	OnStart               func(ctx context.Context, info *callbacks.RunInfo, input *schema.Message) context.Context
	OnEnd                 func(ctx context.Context, info *callbacks.RunInfo, input []*schema.Message) context.Context
	OnEndWithStreamOutput func(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[[]*schema.Message]) context.Context
	OnError               func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context
}

func (ch *ToolsNodeCallbackHandlers) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnEndWithStreamOutput:
		return ch.OnEndWithStreamOutput != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

type AgentCallbackHandler struct {
	OnStart func(ctx context.Context, info *callbacks.RunInfo, input *adk.AgentCallBackInput) context.Context
	OnEnd   func(ctx context.Context, info *callbacks.RunInfo, output *adk.AgentCallbackOutput) context.Context
}

func (ch *AgentCallbackHandler) Needed(ctx context.Context, info *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	default:
		return false
	}
}

type handlerTemplate struct {
	*HandlerHelper
}

func (c *HandlerHelper) Handler() callbacks.Handler {
	return &handlerTemplate{c}
}

func (c *handlerTemplate) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	switch info.Component {
	case components.ComponentOfPrompt:
		return c.promptHandler.OnStart(ctx, info, prompt.ConvCallbackInput(input))
	case components.ComponentOfChatModel:
		return c.chatModelHandler.OnStart(ctx, info, model.ConvCallbackInput(input))
	case components.ComponentOfEmbedding:
		return c.embeddingHandler.OnStart(ctx, info, embedding.ConvCallbackInput(input))
	case components.ComponentOfIndexer:
		return c.indexerHandler.OnStart(ctx, info, indexer.ConvCallbackInput(input))
	case components.ComponentOfRetriever:
		return c.retrieverHandler.OnStart(ctx, info, retriever.ConvCallbackInput(input))
	case components.ComponentOfLoader:
		return c.loaderHandler.OnStart(ctx, info, document.ConvLoaderCallbackInput(input))
	case components.ComponentOfTransformer:
		return c.transformerHandler.OnStart(ctx, info, document.ConvTransformerCallbackInput(input))
	case components.ComponentOfTool:
		return c.toolHandler.OnStart(ctx, info, tool.ConvCallbackInput(input))
	case compose.ComponentOfToolsNode:
		return c.toolsNodeHandler.OnStart(ctx, info, convToolsNodeCallbackInput(input))
	case adk.ComponentOfAgent:
		return c.agentHandler.OnStart(ctx, info, adk.ConvAgentCallbackInput(input))
	case compose.ComponentOfGraph, compose.ComponentOfChain, compose.ComponentOfLambda:
		return c.composeTemplates[info.Component].OnStart(ctx, info, input)
	default:
		return ctx
	}
}

func (c *handlerTemplate) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	switch info.Component {
	case components.ComponentOfPrompt:
		return c.promptHandler.OnEnd(ctx, info, prompt.ConvCallbackOutput(output))
	case components.ComponentOfChatModel:
		return c.chatModelHandler.OnEnd(ctx, info, model.ConvCallbackOutput(output))
	case components.ComponentOfEmbedding:
		return c.embeddingHandler.OnEnd(ctx, info, embedding.ConvCallbackOutput(output))
	case components.ComponentOfIndexer:
		return c.indexerHandler.OnEnd(ctx, info, indexer.ConvCallbackOutput(output))
	case components.ComponentOfRetriever:
		return c.retrieverHandler.OnEnd(ctx, info, retriever.ConvCallbackOutput(output))
	case components.ComponentOfLoader:
		return c.loaderHandler.OnEnd(ctx, info, document.ConvLoaderCallbackOutput(output))
	case components.ComponentOfTransformer:
		return c.transformerHandler.OnEnd(ctx, info, document.ConvTransformerCallbackOutput(output))
	case components.ComponentOfTool:
		return c.toolHandler.OnEnd(ctx, info, tool.ConvCallbackOutput(output))
	case compose.ComponentOfToolsNode:
		return c.toolsNodeHandler.OnEnd(ctx, info, convToolsNodeCallbackOutput(output))
	case adk.ComponentOfAgent:
		return c.agentHandler.OnEnd(ctx, info, adk.ConvAgentCallbackOutput(output))
	case compose.ComponentOfGraph,
		compose.ComponentOfChain,
		compose.ComponentOfLambda:
		return c.composeTemplates[info.Component].OnEnd(ctx, info, output)
	default:
		return ctx
	}
}

func convToolsNodeCallbackInput(src callbacks.CallbackInput) *schema.Message {
	switch t := src.(type) {
	case *schema.Message:
		return t
	default:
		return nil
	}
}

func convToolsNodeCallbackOutput(src callbacks.CallbackInput) []*schema.Message {
	switch t := src.(type) {
	case []*schema.Message:
		return t
	default:
		return nil
	}
}

func (c *handlerTemplate) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	switch info.Component {
	case compose.ComponentOfGraph,
		compose.ComponentOfChain,
		compose.ComponentOfLambda:
		return c.composeTemplates[info.Component].OnStartWithStreamInput(ctx, info, input)
	default:
		return ctx
	}
}

func (c *handlerTemplate) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	switch info.Component {
	case components.ComponentOfChatModel:
		return c.chatModelHandler.OnEndWithStreamOutput(ctx, info,
			schema.StreamReaderWithConvert(output, func(item callbacks.CallbackOutput) (*model.CallbackOutput, error) {
				return model.ConvCallbackOutput(item), nil
			}))
	case components.ComponentOfTool:
		return c.toolHandler.OnEndWithStreamOutput(ctx, info,
			schema.StreamReaderWithConvert(output, func(item callbacks.CallbackOutput) (*tool.CallbackOutput, error) {
				return tool.ConvCallbackOutput(item), nil
			}))
	case compose.ComponentOfToolsNode:
		return c.toolsNodeHandler.OnEndWithStreamOutput(ctx, info,
			schema.StreamReaderWithConvert(output, func(item callbacks.CallbackOutput) ([]*schema.Message, error) {
				return convToolsNodeCallbackOutput(item), nil
			}))
	case compose.ComponentOfGraph,
		compose.ComponentOfChain,
		compose.ComponentOfLambda:
		return c.composeTemplates[info.Component].OnEndWithStreamOutput(ctx, info, output)
	default:
		return ctx
	}
}

// 当前这次回调事件，按组件类型和时机判断，是否值得继续分发下去执行相关回调
func (c *handlerTemplate) Needed(ctx context.Context, info *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	if info == nil {
		return false
	}

	switch info.Component {
	case components.ComponentOfChatModel:
		if c.chatModelHandler != nil && c.chatModelHandler.Needed(ctx, info, timing) {
			return true
		}
	case components.ComponentOfEmbedding:
		if c.embeddingHandler != nil && c.embeddingHandler.Needed(ctx, info, timing) {
			return true
		}
	case components.ComponentOfIndexer:
		if c.indexerHandler != nil && c.indexerHandler.Needed(ctx, info, timing) {
			return true
		}
	case components.ComponentOfLoader:
		if c.loaderHandler != nil && c.loaderHandler.Needed(ctx, info, timing) {
			return true
		}
	case components.ComponentOfPrompt:
		if c.promptHandler != nil && c.promptHandler.Needed(ctx, info, timing) {
			return true
		}
	case components.ComponentOfRetriever:
		if c.retrieverHandler != nil && c.retrieverHandler.Needed(ctx, info, timing) {
			return true
		}
	case components.ComponentOfTool:
		if c.toolHandler != nil && c.toolHandler.Needed(ctx, info, timing) {
			return true
		}
	case components.ComponentOfTransformer:
		if c.transformerHandler != nil && c.transformerHandler.Needed(ctx, info, timing) {
			return true
		}
	case compose.ComponentOfToolsNode:
		if c.toolsNodeHandler != nil && c.toolsNodeHandler.Needed(ctx, info, timing) {
			return true
		}
	case adk.ComponentOfAgent:
		if c.agentHandler != nil && c.agentHandler.Needed(ctx, info, timing) {
			return true
		}
	// 这三个不是基础组件，而是 compose 层的编排型组件
	// 它们的 handler 不是单独的 *XxxCallbackHandler 字段，而是统一塞进 composeTemplates
	// 所以需要把 Needed 请求转发给底层的基础组件
	case compose.ComponentOfGraph,
		compose.ComponentOfChain,
		compose.ComponentOfLambda:
		handler := c.composeTemplates[info.Component]
		if handler != nil {
			checker, ok := handler.(callbacks.TimingChecker)
			if !ok || checker.Needed(ctx, info, timing) {
				return true
			}
		}
	default:
		return false

	}

	// 用于处理不满足各个分支 if 条件的情况
	return false
}

// 直接分发给底层进行执行
func (c *handlerTemplate) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	switch info.Component {
	case components.ComponentOfPrompt:
		return c.promptHandler.OnError(ctx, info, err)
	case components.ComponentOfChatModel:
		return c.chatModelHandler.OnError(ctx, info, err)
	case components.ComponentOfEmbedding:
		return c.embeddingHandler.OnError(ctx, info, err)
	case components.ComponentOfIndexer:
		return c.indexerHandler.OnError(ctx, info, err)
	case components.ComponentOfRetriever:
		return c.retrieverHandler.OnError(ctx, info, err)
	case components.ComponentOfLoader:
		return c.loaderHandler.OnError(ctx, info, err)
	case components.ComponentOfTransformer:
		return c.transformerHandler.OnError(ctx, info, err)
	case components.ComponentOfTool:
		return c.toolHandler.OnError(ctx, info, err)
	case compose.ComponentOfToolsNode:
		return c.toolsNodeHandler.OnError(ctx, info, err)
	case compose.ComponentOfGraph,
		compose.ComponentOfChain,
		compose.ComponentOfLambda:
		return c.composeTemplates[info.Component].OnError(ctx, info, err)
	default:
		return ctx
	}
}
