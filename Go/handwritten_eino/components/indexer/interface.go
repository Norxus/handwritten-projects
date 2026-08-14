package indexer

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

type Indexer interface {
	Store(ctx context.Context, docs []*schema.Document, opts ...Option)
}
