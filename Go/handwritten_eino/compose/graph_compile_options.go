package compose

type graphCompileOptions struct {
	maxRunSteps     int
	graphName       string
	nodeTriggerMode NodeTriggerMode

	callbacks []GraphCompileCallback

	origOpts []GraphCompileOption

	checkPointStore CheckPointStore

	serializer Serializer

	interruptBeforeNodes []string
	interruptAfterNodes  []string

	eagerDisabled bool

	mergeConfig map[string]FanInMergeConfig
}

func newGraphCompileOptions(opts ...GraphCompileOption) *graphCompileOptions {
	option := &graphCompileOptions{}

	for _, o := range opts {
		o(option)
	}

	option.origOpts = opts

	return option
}

// option 模式
type GraphCompileOption func(*graphCompileOptions)

type FanInMergeConfig struct {
	StreamMergeWithSourceEOF bool
}
