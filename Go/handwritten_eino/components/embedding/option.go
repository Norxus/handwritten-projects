package embedding

type Options struct {
	Model *string
}

type Option struct {
	apply func(opts *Options)

	implSpecifixOptFn any
}
