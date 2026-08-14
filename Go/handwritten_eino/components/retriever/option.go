package retriever

type Options struct {
	Index          *string
	SubIndex       *string
	TopK           *int
	ScoreThreshold *float64
	DSLInfo        map[string]any
}

type Option struct {
	apply func(opt *Options)

	implSpecificOptFn any
}
