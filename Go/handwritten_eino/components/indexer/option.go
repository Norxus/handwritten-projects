package indexer

import "github.com/cloudwego/eino/components/embedding"

type Options struct {
	SubIndexes []string
	Embedding  embedding.Embedder
}

type Option struct {
	apply             func(opts *Options)
	implSpecificOptFn any
}
