package callbacks

import "github.com/cloudwego/eino/internal/callbacks"

type RunInfo = callbacks.RunInfo

type CallbackInput = callbacks.CallbackInput

type CallbackOutput = callbacks.CallbackOutput

type Handler = callbacks.Handler

type CallbackTiming = callbacks.CallbackTiming

const (
	// 组件真正开始处理之前触发
	TimingOnStart CallbackTiming = iota
	// 组件成功返回之后触发
	TimingOnEnd
	// 组件返回非 nil 错误触发
	TimingOnError
	// 组件收到的是流式输入时触发
	TimingOnStartWithStreamInput
	// 组件成功返回的是流式输出时触发
	TimingOnEndWithStreamOutput
)

// 在这个包里注册
func InitCallbackHandlers(handlers []Handler) {
	callbacks.GlobalHandlers = handlers
}

func AppendGlobalHandlers(handlers ...Handler) {
	callbacks.GlobalHandlers = append(callbacks.GlobalHandlers, handlers...)
}

type TimingChecker = callbacks.TimingChecker
