package retriever

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

type Retriever interface {
	Retrieve(ctx context.Context, query string, opts ...Option) ([]*schema.Document, error)
}
