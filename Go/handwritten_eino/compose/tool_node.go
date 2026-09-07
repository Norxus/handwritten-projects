package compose

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/internal/callbacks"
	"github.com/cloudwego/eino/internal/safe"
	"github.com/cloudwego/eino/schema"
)

func init() {
	schema.RegisterName[*toolsInterruptAndReturnState]("_eino_compose_tools_interrupt_and_rerun_state")
	schema.RegisterName[*ToolsInterruptAndRerunExtra]("_eino_compose_tools_interrupt_and_rerun_state")
}

type ToolsInterruptAndRerunExtra struct {
	ToolCalls []schema.ToolCall

	ExecutedTools map[string]string

	ExecutedEnhancedTools map[string]*schema.ToolResult

	RerunTools []string

	RerunExtraMap map[string]any
}

type toolsInterruptAndRerunState struct {
	Input                 *schema.Message
	ExecutedTools         map[string]string
	ExecutedEnhancedTools map[string]*schema.ToolResult
	RerunTools            []string
}

type toolsNodeOptions struct {
	ToolOptions []tool.Option
	ToolList    []tool.BaseTool
}

type ToolsNodeOption func(o *toolsNodeOptions)

type ToolsNode struct {
	tuple                             *toolsTuple
	unknownToolHandler                func(ctx context.Context, name, input string) (string, error)
	executeSequentically              bool
	toolArgumentsHandler              func(ctx context.Context, name, input string) (string, error)
	toolCallMiddleWares               []InvokableToolMiddleware
	streamToolCallModdlewares         []StreamableToolMiddleware
	enhanceToolCallMiddlewares        []EnhancedInvokableToolMiddleware
	enhancedStreamToolCallMiddlewares []EnhancedStreamableToolMiddleware
}

type toolsTuple struct {
	indexes                     map[string]int
	meta                        []*executorMeta
	endpoints                   []InvokableToolEndpoint
	streamEndpoints             []StreamableToolEndpoint
	enhancedInvokableEndpoints  []EnhancedInvokableToolEndpoint
	enhancedStreamableEndpoints []EnhancedStreamableToolEndpoint
}

type ToolInput struct {
	Name        string
	Arguments   string
	CallID      string
	CallOptions []tool.Option
}

type ToolOutput struct {
	Result string
}

type StreamToolOutput struct {
	Result *schema.StreamReader[string]
}

type EnhancedInvokableToolOutput struct {
	Result *schema.ToolResult
}

type EnhancedStreamableToolOutput struct {
	Result *schema.StreamReader[*schema.ToolResult]
}

type InvokableToolEndpoint func(ctx context.Context, input *ToolInput) (*ToolOutput, error)

type StreamableToolEndpoint func(ctx context.Context, input *ToolInput) (*StreamToolOutput, error)

type EnhancedInvokableToolEndpoint func(ctx context.Context, input *ToolInput) (*EnhancedInvokableToolOutput, error)

type EnhancedStreamableToolEndpoint func(ctx context.Context, input *ToolInput) (*EnhancedStreamableToolOutput, error)

type InvokableToolMiddleware func(InvokableToolEndpoint) InvokableToolEndpoint

type StreamableToolMiddleware func(StreamableToolEndpoint) StreamableToolEndpoint

type EnhancedInvokableToolMiddleware func(EnhancedInvokableToolEndpoint) EnhancedInvokableToolEndpoint

type EnhancedStreamableToolMiddleware func(EnhancedStreamableToolEndpoint) EnhancedStreamableToolEndpoint

type ToolMiddleware struct {
	Invokable InvokableToolMiddleware

	Streamable StreamableToolMiddleware

	EnhancedInvokable EnhancedInvokableToolMiddleware

	EnhancedStreamable EnhancedStreamableToolMiddleware
}

type ToolsNodeConfig struct {
	Tools []tool.BaseTool

	UnknownToolsHandler func(ctx context.Context, name, input string) (string, error)

	ExecuteSequentially bool

	ToolArgumentsHandler func(ctx context.Context, name, arguments string) (string, error)

	ToolCallMiddlewares []ToolMiddleware
}

type toolsInterruptAndReturnState struct {
	Input                 *schema.Message
	ExecutedTools         map[string]string
	ExecutedEnhancedTools map[string]*schema.ToolResult
	ReturnTools           []string
}

type toolCallTask struct {
	endpoint                   InvokableToolEndpoint
	streamEndpoint             StreamableToolEndpoint
	enhancedInvokableEndpoint  EnhancedInvokableToolEndpoint
	enhancedStreamableEndpoint EnhancedStreamableToolEndpoint
	meta                       *executorMeta
	name                       string
	arg                        string
	callID                     string
	useEnhanced                bool

	executed        bool
	output          string
	sOutput         *schema.StreamReader[string]
	enhancedOutput  *schema.ToolResult
	enhancedSOutput *schema.StreamReader[*schema.ToolResult]
	err             error
}

// 把原始 tool 列表整理成统一、可索引、带中间件、可互相兜底转换的内部执行入口集合 toolsTuple
func convTools(ctx context.Context, tools []tool.BaseTool, ms []InvokableToolMiddleware, sms []StreamableToolMiddleware,
	ems []EnhancedInvokableToolMiddleware, esms []EnhancedStreamableToolMiddleware) (*toolsTuple, error) {
	ret := &toolsTuple{
		indexes:                     make(map[string]int),
		meta:                        make([]*executorMeta, len(tools)),
		endpoints:                   make([]InvokableToolEndpoint, len(tools)),
		streamEndpoints:             make([]StreamableToolEndpoint, len(tools)),
		enhancedInvokableEndpoints:  make([]EnhancedInvokableToolEndpoint, len(tools)),
		enhancedStreamableEndpoints: make([]EnhancedStreamableToolEndpoint, len(tools)),
	}

	for idx, bt := range tools {
		tl, err := bt.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("(NewToolNode) failed to get tool info at idx=%d: %w", idx, err)
		}

		toolName := tl.Name
		var (
			st     tool.StreamableTool
			it     tool.InvokableTool
			eiTool tool.EnhancedInvokableTool
			esTool tool.EnhancedStreamableTool

			invokable          InvokableToolEndpoint
			streamable         StreamableToolEndpoint
			enhancedInvokable  EnhancedInvokableToolEndpoint
			enhancedStreamable EnhancedStreamableToolEndpoint

			ok   bool
			meta *executorMeta
		)

		meta = parseExecutorInfoFromComponent(components.ComponentOfTool, bt)

		// 判断这个 tool 实现了哪些能力，把对应能力包装成统一 endpoint，并套上 middleware
		if st, ok := bt.(tool.StreamableTool); ok {
			streamable = wrapStreamToolCall(st, sms, !meta.isComponentCallbackEnabled)
		}

		if it, ok = bt.(tool.InvokableTool); ok {
			invokable = wrapToolCall(it, ms, !meta.isComponentCallbackEnabled)
		}

		if eiTool, ok = bt.(tool.EnhancedInvokableTool); ok {
			enhancedInvokable = wrapEnhancedInvokableToolCall(eiTool, ems, !meta.isComponentCallbackEnabled)
		}

		if esTool, ok = bt.(tool.EnhancedStreamableTool); ok {
			enhancedStreamable = wrapEnhancedStreamableToolCall(esTool, esms, !meta.isComponentCallbackEnabled)
		}

		if st == nil && it == nil && eiTool == nil && esTool == nil {
			return nil, fmt.Errorf("tool %s is not invokable, streamable, enhanced invokable or enhanced streamable", toolName)
		}
		// 转换补齐能力
		if streamable == nil && invokable != nil {
			streamable = invokableToStreamable(invokable)
		}
		if invokable == nil && streamable != nil {
			invokable = streamableToInvokable(streamable)
		}

		if enhancedStreamable == nil && enhancedInvokable != nil {
			enhancedStreamable = enhancedInvokableToEnhancedStreamable(enhancedInvokable)
		}
		if enhancedInvokable == nil && enhancedStreamable != nil {
			enhancedInvokable = enhancedStreamableToEnhancedInvokable(enhancedStreamable)
		}

		ret.indexes[toolName] = idx
		ret.meta[idx] = meta
		ret.endpoints[idx] = invokable
		ret.streamEndpoints[idx] = streamable
		ret.enhancedInvokableEndpoints[idx] = enhancedInvokable
		ret.enhancedStreamableEndpoints[idx] = enhancedStreamable
	}

	return ret, nil
}

// 把 非流式 工具调用包装成 流式 工具调用
func invokableToStreamable(e InvokableToolEndpoint) StreamableToolEndpoint {
	return func(ctx context.Context, input *ToolInput) (*StreamToolOutput, error) {
		o, err := e(ctx, input)
		if err != nil {
			return nil, err
		}
		return &StreamToolOutput{Result: schema.StreamReaderFromArray([]string{o.Result})}, nil
	}
}

// 流式处理转非流式处理
func streamableToInvokable(e StreamableToolEndpoint) InvokableToolEndpoint {
	return func(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
		so, err := e(ctx, input)
		if err != nil {
			return nil, err
		}
		o, err := concatStreamReader(so.Result)
		if err != nil {
			return nil, fmt.Errorf("failed to concat StreamableTool output message stream: %w", err)
		}
		return &ToolOutput{Result: o}, nil
	}
}

func enhancedStreamableToEnhancedInvokable(e EnhancedStreamableToolEndpoint) EnhancedInvokableToolEndpoint {
	return func(ctx context.Context, input *ToolInput) (*EnhancedInvokableToolOutput, error) {
		so, err := e(ctx, input)
		if err != nil {
			return nil, err
		}
		o, err := concatStreamReader(so.Result)
		if err != nil {
			return nil, fmt.Errorf("failed to concat EnhancedStreamableTool output message stream: %w", err)
		}
		return &EnhancedInvokableToolOutput{Result: o}, nil
	}
}

func enhancedInvokableToEnhancedStreamable(e EnhancedInvokableToolEndpoint) EnhancedStreamableToolEndpoint {
	return func(ctx context.Context, input *ToolInput) (*EnhancedStreamableToolOutput, error) {
		o, err := e(ctx, input)
		if err != nil {
			return nil, err
		}
		return &EnhancedStreamableToolOutput{Result: schema.StreamReaderFromArray([]*schema.ToolResult{o.Result})}, nil
	}
}

type invokableToolWithCallback struct {
	it tool.InvokableTool
}

func (i *invokableToolWithCallback) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return i.it.Info(ctx)
}

func (i *invokableToolWithCallback) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	return invokeWithCallbacks(i.it.InvokableRun)(ctx, argumentsInJSON, opts...)
}

func wrapToolCall(it tool.InvokableTool, middlewares []InvokableToolMiddleware, needCallback bool) InvokableToolEndpoint {
	// 倒序洋葱执行中间件
	middleware := func(next InvokableToolEndpoint) InvokableToolEndpoint {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}

		return next
	}

	// 如果需要 callback，自动补齐中间件
	if needCallback {
		it = &invokableToolWithCallback{it: it}
	}

	return middleware(func(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
		result, err := it.InvokableRun(ctx, input.Arguments, input.CallOptions...)
		if err != nil {
			return nil, err
		}
		return &ToolOutput{Result: result}, nil
	})
}

type streamableToolWithCallback struct {
	st tool.StreamableTool
}

func (s *streamableToolWithCallback) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return s.st.Info(ctx)
}

func (s *streamableToolWithCallback) StreamableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (*schema.StreamReader[string], error) {
	return streamWithCallbacks(s.st.StreamableRun)(ctx, argumentsInJSON, opts...)
}

// 包上中间件和 callback
func wrapStreamToolCall(st tool.StreamableTool, middlewares []StreamableToolMiddleware, needCallback bool) StreamableToolEndpoint {
	middleware := func(next StreamableToolEndpoint) StreamableToolEndpoint {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}

	if needCallback {
		st = &streamableToolWithCallback{st: st}
	}

	return middleware(func(ctx context.Context, input *ToolInput) (*StreamToolOutput, error) {
		result, err := st.StreamableRun(ctx, input.Arguments, input.CallOptions...)
		if err != nil {
			return nil, err
		}
		return &StreamToolOutput{Result: result}, nil
	})
}

type enhancedInvokableToolWithCallback struct {
	eiTool tool.EnhancedInvokableTool
}

func (e *enhancedInvokableToolWithCallback) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return e.eiTool.Info(ctx)
}

func (e *enhancedInvokableToolWithCallback) InvokableRun(ctx context.Context, toolArgument *schema.ToolArgument, opts ...tool.Option) (*schema.ToolResult, error) {
	return invokeEnhancedWithCallbacks(e.eiTool.InvokableRun)(ctx, toolArgument, opts...)
}

func invokeEnhancedWithCallbacks(i func(ctx context.Context, toolArgument *schema.ToolArgument, opts ...tool.Option) (*schema.ToolResult, error)) func(ctx context.Context, toolArgument *schema.ToolArgument, opts ...tool.Option) (*schema.ToolResult, error) {
	return runWithCallbacks(i, onStart[*schema.ToolArgument], onEnd[*schema.ToolResult], onError)
}

func streamEnhancedWithCallbacks(s func(ctx context.Context, toolArgument *schema.ToolArgument, opts ...tool.Option) (*schema.StreamReader[*schema.ToolResult], error)) func(ctx context.Context, toolArgument *schema.ToolArgument, opts ...tool.Option) (*schema.StreamReader[*schema.ToolResult], error) {
	return runWithCallbacks(s, onStart[*schema.ToolArgument], onEndWithStreamOutput[*schema.ToolResult], onError)
}

func wrapEnhancedInvokableToolCall(eiTool tool.EnhancedInvokableTool, middlewares []EnhancedInvokableToolMiddleware, needCallback bool) EnhancedInvokableToolEndpoint {
	middleware := func(next EnhancedInvokableToolEndpoint) EnhancedInvokableToolEndpoint {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}

	if needCallback {
		eiTool = &enhancedInvokableToolWithCallback{eiTool: eiTool}
	}

	return middleware(func(ctx context.Context, input *ToolInput) (*EnhancedInvokableToolOutput, error) {
		result, err := eiTool.InvokableRun(ctx, &schema.ToolArgument{Text: input.Arguments}, input.CallOptions...)
		if err != nil {
			return nil, err
		}
		return &EnhancedInvokableToolOutput{Result: result}, nil
	})
}

type enhancedStreamableToolWithCallback struct {
	est tool.EnhancedStreamableTool
}

func (e *enhancedStreamableToolWithCallback) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return e.est.Info(ctx)
}

func (e *enhancedStreamableToolWithCallback) StreamableRun(ctx context.Context, toolArgument *schema.ToolArgument, opts ...tool.Option) (*schema.StreamReader[*schema.ToolResult], error) {
	return streamEnhancedWithCallbacks(e.est.StreamableRun)(ctx, toolArgument, opts...)
}

func wrapEnhancedStreamableToolCall(est tool.EnhancedStreamableTool, middlewares []EnhancedStreamableToolMiddleware, needCallback bool) EnhancedStreamableToolEndpoint {
	middleware := func(next EnhancedStreamableToolEndpoint) EnhancedStreamableToolEndpoint {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
	if needCallback {
		est = &enhancedStreamableToolWithCallback{est: est}
	}
	return middleware(func(ctx context.Context, input *ToolInput) (*EnhancedStreamableToolOutput, error) {
		result, err := est.StreamableRun(ctx, &schema.ToolArgument{Text: input.Arguments}, input.CallOptions...)
		if err != nil {
			return nil, err
		}
		return &EnhancedStreamableToolOutput{Result: result}, nil
	})
}

// 接收一个带 ToolCalls 的 assistant 消息，执行里面声明的工具调用，最后返回一组 tool role 的消息，顺序和输入里的 ToolCalls 顺序一致
func (tn *ToolsNode) Invoke(ctx context.Context, input *schema.Message,
	opts ...ToolsNodeOption) ([]*schema.Message, error) {

	// 默认使用 tn.tuple 里初始化好的工具集合。
	// 如果调用方通过 WithToolList(...) 临时传了工具列表，就会在这次调用里重新 convTools，生成本次专用的工具索引和执行 endpoint
	opt := getToolsNodeOptions(opts...)
	tuple := tn.tuple
	if opt.ToolList != nil {
		var err error
		tuple, err = convTools(ctx, opt.ToolList, tn.toolCallMiddleWares, tn.streamToolCallModdlewares, tn.enhanceToolCallMiddlewares, tn.enhancedStreamToolCallMiddlewares)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool list from call option: %w", err)
		}
	}

	// 处理中断恢复状态
	// ToolsNode 可能一次执行多个 tool call，其中某些工具成功了，某些工具触发了中断。
	// 恢复执行时，它不能把已经成功的工具再跑一遍，否则工具调用可能产生副作用，比如重复写库、重复发请求、重复扣费
	var executedTools map[string]string
	var executedEnhancedTools map[string]*schema.ToolResult
	// 从 context 中获取执行状态
	if wasInterrupted, hasState, tnState := GetInterruptState[*toolsInterruptAndReturnState](ctx); wasInterrupted && hasState {
		input = tnState.Input
		if tnState.ExecutedTools != nil {
			executedTools = tnState.ExecutedTools
		}
		if tnState.ExecutedEnhancedTools != nil {
			executedEnhancedTools = tnState.ExecutedEnhancedTools
		}
	}

	// genToolCallTasks 会校验输入必须是 Assistant 消息，并要求里面有 ToolCalls。
	// 每个 ToolCall 会被转成一个 toolCallTask。
	// 如果某个 callID 已经存在于 executedTools 或 executedEnhancedTools，这个 task 会直接标记为 executed = true，
	// 不会再次调用真实工具
	tasks, err := tn.genToolCallTasks(ctx, tuple, input, executedTools, executedEnhancedTools, false)
	if err != nil {
		return nil, err
	}

	// 执行这些工具
	if tn.executeSequentically {
		sequentialRunToolCall(ctx, runToolCallTaskByInvoke, tasks, opt.ToolOptions...)
	} else {
		parallelRunToolCall(ctx, runToolCallTaskByInvoke, tasks, opt.ToolOptions...)
	}

	n := len(tasks)
	output := make([]*schema.Message, n)

	// 结果收集和中断处理
	// 偏外部可见的信息，包含原始 tool calls、哪些工具已执行、哪些需要 rerun、每个 rerun 工具的额外 interrupt info
	rerunExtra := &ToolsInterruptAndRerunExtra{
		ToolCalls:             input.ToolCalls,
		ExecutedTools:         make(map[string]string),
		ExecutedEnhancedTools: make(map[string]*schema.ToolResult),
		RerunExtraMap:         make(map[string]any),
	}
	// 内部恢复状态，用于下次 resume 时避免重复执行已成功工具
	rerunState := &toolsInterruptAndRerunState{
		Input:                 input,
		ExecutedTools:         make(map[string]string),
		ExecutedEnhancedTools: make(map[string]*schema.ToolResult),
	}

	// 遍历执行完成的 tasks，收集信息和错误
	var errs []error
	for i := 0; i < n; i++ {
		if tasks[i].err != nil {
			// 普通错误直接返回，中断错误不立即返回，而是收集起来，最后统一 CompositeInterrupt
			info, ok := IsInterruptRerunError(tasks[i].err)
			if !ok {
				return nil, fmt.Errorf("failed to invoke took[name:%s id:%s]: %w", tasks[i].name, tasks[i].callID, tasks[i].err)
			}

			rerunExtra.RerunTools = append(rerunExtra.RerunTools, tasks[i].callID)
			rerunState.RerunTools = append(rerunState.RerunTools, tasks[i].callID)
			if info != nil {
				rerunExtra.RerunExtraMap[tasks[i].callID] = info
			}

			// 给这个中断补上工具级地址，这样恢复系统能知道具体是哪个 tool call 中断了
			iErr := WrapInterruptAndRerunIfNeeded(ctx,
				AddressSegment{ID: tasks[i].callID, Type: AddressSegmentTool}, tasks[i].err)
			errs = append(errs, iErr)
			continue
		}
		// 执行成功了，把结果写进恢复状态
		if tasks[i].executed {
			if tasks[i].useEnhanced {
				rerunExtra.ExecutedEnhancedTools[tasks[i].callID] = tasks[i].enhancedOutput
				rerunState.ExecutedEnhancedTools[tasks[i].callID] = tasks[i].enhancedOutput
			} else {
				rerunExtra.ExecutedTools[tasks[i].callID] = tasks[i].output
				rerunState.ExecutedTools[tasks[i].callID] = tasks[i].output
			}
		}

		// 如果当前还没有任何 interrupt，则把成功结果转成输出消息
		if len(errs) == 0 {
			// 增强工具的内容不放在普通 Content string 里，而是转成 UserInputMultiContent，用于承载文本、图片、音频、视频、文件等结构化多模态结果
			if tasks[i].useEnhanced {
				output[i] = schema.ToolMessage("", tasks[i].callID, schema.WithToolName(tasks[i].name))
				output[i].UserInputMultiContent, err = tasks[i].enhancedOutput.ToMessageInputParts()
				if err != nil {
					return nil, err
				} else {
					output[i] = schema.ToolMessage(tasks[i].output, tasks[i].callID, schema.WithToolName(tasks[i].name))
				}
			}
		}
	}
	// 只要任意工具触发了 interrupt/rerun，整个 ToolsNode.Invoke 就不会返回部分 tool messages，
	// 而是返回一个组合中断错误。这个错误里保存了“哪些工具已经成功、哪些工具需要恢复后重跑”的信息
	if len(errs) > 0 {
		return nil, CompositeInterrupt(ctx, rerunExtra, rerunState, errs...)
	}

	return output, nil
}

func (tn *ToolsNode) Stream(ctx context.Context, input *schema.Message,
	opts ...ToolsNodeOption) (*schema.StreamReader[[]*schema.Message], error) {
	opt := getToolsNodeOptions(opts...)
	tuple := tn.tuple
	// 如果传了 WithToolList，会临时把这批工具转成可执行的 toolsTuple
	if opt.ToolList != nil {
		var err error
		tuple, err = convTools(ctx, opt.ToolList, tn.toolCallMiddleWares, tn.streamToolCallModdlewares, tn.enhanceToolCallMiddlewares, tn.enhancedStreamToolCallMiddlewares)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool list from call option: %w", err)
		}
	}

	var executedTools map[string]string
	var executedEnhancedTools map[string]*schema.ToolResult
	// 如果当前是 interrupt/rerun 恢复流程，会从 ctx 里取回上次的输入和已经执行过的工具结果，避免重复执行
	if wasInterrupted, hasState, tnState := GetInterruptState[*toolsInterruptAndRerunState](ctx); wasInterrupted && hasState {
		input = tnState.Input
		if tnState.ExecutedTools != nil {
			executedTools = tnState.ExecutedTools
		}
		if tnState.ExecutedEnhancedTools != nil {
			executedEnhancedTools = tnState.ExecutedEnhancedTools
		}
	}

	// 是把模型输出里的 input.ToolCalls 转成 ToolsNode 内部可执行任务列表的函数
	tasks, err := tn.genToolCallTasks(ctx, tuple, input, executedTools, executedEnhancedTools, true)
	if err != nil {
		return nil, err
	}

	if tn.executeSequentically {
		sequentialRunToolCall(ctx, runToolCallTaskByStream, tasks, opt.ToolOptions...)
	} else {
		parallelRunToolCall(ctx, runToolCallTaskByStream, tasks, opt.ToolOptions...)
	}

	n := len(tasks)

	rerunExtra := &ToolsInterruptAndRerunExtra{
		ToolCalls:             input.ToolCalls,
		ExecutedTools:         make(map[string]string),
		ExecutedEnhancedTools: make(map[string]*schema.ToolResult),
		RerunExtraMap:         make(map[string]any),
	}
	rerunState := &toolsInterruptAndRerunState{
		Input:                 input,
		ExecutedTools:         make(map[string]string),
		ExecutedEnhancedTools: make(map[string]*schema.ToolResult),
	}

	var errs []error
	for i := 0; i < n; i++ {
		if tasks[i].err != nil {
			info, ok := IsInterruptRerunError(tasks[i].err)
			if !ok {
				return nil, fmt.Errorf("failed to stream tool call %s: %w", tasks[i].callID, tasks[i].err)
			}

			rerunExtra.RerunTools = append(rerunExtra.RerunTools, tasks[i].callID)
			rerunState.RerunTools = append(rerunState.RerunTools)
			if info != nil {
				rerunExtra.RerunExtraMap[tasks[i].callID] = info
			}
			iErr := WrapInterruptAndRerunIfNeeded(ctx,
				AddressSegment{ID: tasks[i].callID, Type: AddressSegmentTool}, tasks[i].err)
			errs = append(errs, iErr)
			continue
		}
	}

	// 本轮 tool stream 执行过程中，至少有一个 tool 返回了 interrupt/rerun 错误
	if len(errs) > 0 {
		for _, t := range tasks {
			// 已经执行成功的 task，记录它们的执行结构，避免整体重复执行时再重跑一遍
			if t.executed {
				if t.useEnhanced {
					eo, err_ := concatStreamReader(t.enhancedSOutput)
					if err_ != nil {
						return nil, fmt.Errorf("failed to concat enhanced tool[name:%s, id:%s]'s stream output: %w", t.name, t.callID, err_)
					}
					rerunExtra.ExecutedEnhancedTools[t.callID] = eo
					rerunState.ExecutedEnhancedTools[t.callID] = eo
				} else {
					o, err_ := concatStreamReader(t.sOutput)
					if err_ != nil {
						return nil, fmt.Errorf("failed to concat tool[name:%s id:%s]'s stream output: %w", t.name, t.callID, err_)
					}
					rerunExtra.ExecutedTools[t.callID] = o
					rerunState.ExecutedTools[t.callID] = o
				}
			}
		}
		return nil, CompositeInterrupt(ctx, rerunExtra, rerunState, errs...)
	}

	sOutput := make([]*schema.StreamReader[[]*schema.Message], n)
	for i := 0; i < n; i++ {
		index := i
		callID := tasks[i].callID
		callName := tasks[i].name
		if tasks[i].useEnhanced {
			// 定义转换函数：把每个流式产出的 *schema.ToolResult 转成 []*schema.Message
			cvt := func(tr *schema.ToolResult) ([]*schema.Message, error) {
				// 通过 index 保留 toolCall 结果的位置关系
				ret := make([]*schema.Message, n)
				ret[index] = schema.ToolMessage("", callID, schema.WithToolName(callName))
				ret[index].UserInputMultiContent, err = tr.ToMessageInputParts()
				if err != nil {
					return nil, err
				}
				return ret, nil
			}
			sOutput[i] = schema.StreamReaderWithConvert(tasks[i].enhancedSOutput, cvt)
		} else {
			cvt := func(s string) ([]*schema.Message, error) {
				ret := make([]*schema.Message, n)
				ret[index] = schema.ToolMessage(s, callID, schema.WithToolName(callName))
				return ret, nil
			}
			sOutput[i] = schema.StreamReaderWithConvert(tasks[i].sOutput, cvt)
		}
	}

	// 合并所有工具的输出流进行输出
	return schema.MergeStreamReaders(sOutput), nil
}

// 执行一个工具调用任务的“流式版本”，并把执行结果或错误写回 toolCallTask 结构体
func runToolCallTaskByStream(ctx context.Context, task *toolCallTask, opts ...tool.Option) {
	// 把当前的工具调用信息写入 context
	ctx = callbacks.ReuseHandlers(ctx, &callbacks.RunInfo{
		Name:      task.name,
		Type:      task.meta.componentImplType,
		Component: task.meta.component,
	})

	ctx = setToolCallInfo(ctx, &toolCallInfo{toolCallID: task.callID})
	ctx = appendToolAddressSegment(ctx, task.name, task.callID)

	// 执行 tool 内部的执行函数，将结果和错误会写回 task 中
	// EnhancedStreamableToolEndpoint = 包装后的 EnhancedStreamableTool.StreamableRun，里面封装了中间件
	if task.useEnhanced {
		enhancedOutput, err := task.enhancedStreamableEndpoint(ctx, &ToolInput{
			Name:        task.name,
			Arguments:   task.arg,
			CallID:      task.callID,
			CallOptions: opts,
		})
		if err != nil {
			task.err = err
		} else {
			task.enhancedSOutput = enhancedOutput.Result
			task.executed = true
		}
	} else {
		output, err := task.streamEndpoint(ctx, &ToolInput{
			Name:        task.name,
			Arguments:   task.arg,
			CallID:      task.callID,
			CallOptions: opts,
		})
		if err != nil {
			task.err = err
		}else {
			task.sOutput = output.Result
			task.executed = true
		}
	}
}

func runToolCallTaskByInvoke(ctx context.Context, task *toolCallTask, opts ...tool.Option) {
	if task.executed {
		return
	}
	// 把 tool 信息写入 ctx 中，让 callback 链知道当前正在执行哪个 tool
	ctx = callbacks.ReuseHandlers(ctx, &callbacks.RunInfo{
		Name:      task.name,
		Type:      task.meta.componentImplType,
		Component: task.meta.component,
	})

	// 把 toolCall 信息写入 ctx 中，后续 tool 内部或框架逻辑可以通过 context 识别“当前是哪个 tool call
	ctx = setToolCallInfo(ctx, &toolCallInfo{toolCallID: task.callID})
	// 记录地址信息
	ctx = appendToolAddressSegment(ctx, task.name, task.callID)

	// 开始调用 tool，把结果和错误都写入 task 中，不会返回这些信息
	if task.useEnhanced {
		enhancedOutput, err := task.enhancedInvokableEndpoint(ctx, &ToolInput{
			Name:        task.name,
			Arguments:   task.arg,
			CallID:      task.callID,
			CallOptions: opts,
		})
		if err != nil {
			task.err = err
		} else {
			task.enhancedOutput = enhancedOutput.Result
			task.executed = true
		}
	} else {
		output, err := task.endpoint(ctx, &ToolInput{
			Name:        task.name,
			Arguments:   task.arg,
			CallID:      task.callID,
			CallOptions: opts,
		})
		if err != nil {
			task.err = err
		} else {
			task.output = output.Result
			task.executed = true
		}
	}
}

// for 循环顺序执行 toolCallTask
func sequentialRunToolCall(ctx context.Context,
	run func(ctx2 context.Context, callTask *toolCallTask, opts ...tool.Option),
	tasks []toolCallTask, opts ...tool.Option) {

	for i := range tasks {
		if tasks[i].executed {
			continue
		}
		run(ctx, &tasks[i], opts...)
	}
}

func parallelRunToolCall(ctx context.Context,
	run func(ctx2 context.Context, callTask *toolCallTask, opts ...tool.Option),
	tasks []toolCallTask, opts ...tool.Option) {
	if len(tasks) == 1 {
		run(ctx, &tasks[0], opts...)
		return
	}

	// 第 0 个任务在当前 gorutine 执行，少起一个 goroutine
	var wg sync.WaitGroup
	for i := 1; i < len(tasks); i++ {
		if tasks[i].executed {
			continue
		}

		wg.Add(1)
		go func(ctx_ context.Context, t *toolCallTask, opts ...tool.Option) {
			defer wg.Done()

			defer func() {
				panicErr := recover()
				if panicErr != nil {
					t.err = safe.NewPanicErr(panicErr, debug.Stack())
				}
			}()
			run(ctx_, t, opts...)
		}(ctx, &tasks[i], opts...)
	}

	if !tasks[0].executed {
		run(ctx, &tasks[0], opts...)
	}

	wg.Wait()
}

// 把 assistant message 里的 tool_calls 解析成可执行任务 （toolCallTask）列表
func (tn *ToolsNode) genToolCallTasks(ctx context.Context, tuple *toolsTuple,
	input *schema.Message, executedTools map[string]string, executedEnhancedTools map[string]*schema.ToolResult, isStream bool) ([]toolCallTask, error) {
	// 输入校验
	if input.Role != schema.Assistant {
		return nil, fmt.Errorf("expected message role is Assistant, got %s", input.Role)
	}

	n := len(input.ToolCalls)
	if n == 0 {
		return nil, errors.New("no tool call found in input message")
	}

	// 为每个 toolCall 生成一个 toolCallTask
	toolCallTasks := make([]toolCallTask, n)
	for i := 0; i < n; i++ {
		toolCall := input.ToolCalls[i]
		// 已执行过的 tool 就直接把结果放入 task 中，并标记 executed = true，避免重复执行
		if enhancedResult, executed := executedEnhancedTools[toolCall.ID]; executed {
			toolCallTasks[i].name = toolCall.Function.Name
			toolCallTasks[i].arg = toolCall.Function.Arguments
			toolCallTasks[i].callID = toolCall.ID
			toolCallTasks[i].executed = true
			toolCallTasks[i].useEnhanced = true
			if isStream {
				toolCallTasks[i].enhancedSOutput = schema.StreamReaderFromArray([]*schema.ToolResult{enhancedResult})
			} else {
				toolCallTasks[i].enhancedOutput = enhancedResult
			}
			continue
		}

		if result, executed := executedTools[toolCall.ID]; executed {
			toolCallTasks[i].name = toolCall.Function.Name
			toolCallTasks[i].arg = toolCall.Function.Arguments
			toolCallTasks[i].callID = toolCall.ID
			toolCallTasks[i].executed = true
			toolCallTasks[i].useEnhanced = false
			if isStream {
				toolCallTasks[i].sOutput = schema.StreamReaderFromArray([]string{result})
			} else {
				toolCallTasks[i].output = result
			}
			continue
		}

		// tuple 用来保存 ToolsNode 当前可用工具的索引表和执行入口集合
		// 模型说要调用某个工具名，tuple 负责告诉框架这个工具是否存在，以及它在各个 endpoint 数组里的下标是多少
		index, ok := tuple.indexes[toolCall.Function.Name]
		if !ok {
			if tn.unknownToolHandler == nil {
				return nil, fmt.Errorf("tool %s not found in toolsNode indexes", toolCall.Function.Name)
			}
			// 没拿到，就构造一个兜底任务
			toolCallTasks[i] = newUnknownToolTask(toolCall.Function.Name, toolCall.Function.Arguments, toolCall.ID, tn.unknownToolHandler)
		} else {
			// 找到了就填充工具元信息
			toolCallTasks[i].meta = tuple.meta[index]
			toolCallTasks[i].name = toolCall.Function.Name
			toolCallTasks[i].callID = toolCall.ID

			if tuple.enhancedInvokableEndpoints[index] != nil && tuple.enhancedStreamableEndpoints[index] != nil {
				toolCallTasks[i].enhancedInvokableEndpoint = tuple.enhancedInvokableEndpoints[index]
				toolCallTasks[i].enhancedStreamableEndpoint = tuple.enhancedStreamableEndpoints[index]
				toolCallTasks[i].useEnhanced = true
			} else {
				toolCallTasks[i].endpoint = tuple.endpoints[index]
				toolCallTasks[i].streamEndpoint = tuple.streamEndpoints[index]
				toolCallTasks[i].useEnhanced = false
			}

			// 如果配置了 tn.toolArgumentsHandler，就对使用该 handler 做参数的预加工
			if tn.toolArgumentsHandler != nil {
				arg, err := tn.toolArgumentsHandler(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
				if err != nil {
					return nil, fmt.Errorf("failed to executed tool[name: %s arguments: %s] argument handler: %w", toolCall.Function.Name, toolCall.Function.Arguments, err)
				}
				toolCallTasks[i].arg = arg
			} else {
				toolCallTasks[i].arg = toolCall.Function.Arguments
			}
		}

	}
	return toolCallTasks, nil
}

// 把“未知工具”的兜底处理逻辑，包装成一个标准的 toolCallTask
func newUnknownToolTask(name, arg, callID string, unknownToolHandler func(ctx context.Context, name, input string) (string, error)) toolCallTask {
	// 适配成标准的接口
	endpoint := func(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
		result, err := unknownToolHandler(ctx, input.Name, input.Arguments)
		if err != nil {
			return nil, err
		}
		return &ToolOutput{
			Result: result,
		}, nil
	}
	return toolCallTask{
		endpoint:       endpoint,
		streamEndpoint: invokableToStreamable(endpoint),
		meta: &executorMeta{
			component:                  components.ComponentOfTool,
			isComponentCallbackEnabled: false,
			componentImplType:          "UnknownTool",
		},
		name:   name,
		arg:    arg,
		callID: callID,
	}
}

func getToolsNodeOptions(opts ...ToolsNodeOption) *toolsNodeOptions {
	o := &toolsNodeOptions{
		ToolOptions: make([]tool.Option, 0),
	}

	for _, opt := range opts {
		opt(o)
	}
	return o
}

type toolCallInfoKey struct{}
type toolCallInfo struct {
	toolCallID string
}

// 用 context 存储 callInfo 信息
func setToolCallInfo(ctx context.Context, toolCallInfo *toolCallInfo) context.Context {
	return context.WithValue(ctx, toolCallInfoKey{}, toolCallInfo)
}

func GetToolCallID(ctx context.Context) string {
	v := ctx.Value(toolCallInfoKey{})
	if v == nil {
		return ""
	}

	info, ok := v.(*toolCallInfo)
	if !ok {
		return ""
	}

	return info.toolCallID
}
