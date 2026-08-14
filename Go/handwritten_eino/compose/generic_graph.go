package compose

import (
	"context"
	"reflect"
)

type newGraphOptions struct {
	withState func(ctx context.Context) any
	stateType reflect.Type
}

type NewGraphOption func(ngo *newGraphOptions)
