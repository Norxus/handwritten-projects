package document

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

type Source struct {
	URI string
}

type Loader interface {
	Load(ctx context.Context, src Source, opts ...Loader)
}

type Transformer interface {
	Transform(ctx context.Context, src []*schema.Document, opts ...TransformerOption) ([]*schema.Document, error)
}
