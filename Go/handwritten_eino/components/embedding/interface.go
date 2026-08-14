package embedding

import "context"

type Embedder interface {
	EmbedStrings(ctx context.Context, texts []string, opts ...Option)([][]float64, error)
}