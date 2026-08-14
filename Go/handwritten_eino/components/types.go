package components

type Component string

// 这个运行中的东西属于哪一类组件
const (
	// ComponentOfPrompt identifies chat template components.
	ComponentOfPrompt Component = "ChatTemplate"
	// ComponentOfChatModel identifies chat model components.
	ComponentOfChatModel Component = "ChatModel"
	// ComponentOfEmbedding identifies embedding components.
	ComponentOfEmbedding Component = "Embedding"
	// ComponentOfIndexer identifies indexer components.
	ComponentOfIndexer Component = "Indexer"
	// ComponentOfRetriever identifies retriever components.
	ComponentOfRetriever Component = "Retriever"
	// ComponentOfLoader identifies loader components.
	ComponentOfLoader Component = "Loader"
	// ComponentOfTransformer identifies document transformer components.
	ComponentOfTransformer Component = "DocumentTransformer"
	// ComponentOfTool identifies tool components.
	ComponentOfTool Component = "Tool"
)

type Typer interface {
	GetType() string
}

func GetType(component any) (string, bool) {
	if typer, ok := component.(Typer); ok {
		return typer.GetType(), true
	}

	return "", false
}

type Checker interface {
	IsCallbacksEnabled() bool
}

func IsCallbacksEnabled(i any) bool {
	if checker, ok := i.(Checker); ok {
		return checker.IsCallbacksEnabled()
	}
	return false
}