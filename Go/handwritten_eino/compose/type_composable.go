package compose

import (
	"context"
	"reflect"
)

type AnyGraph interface {
	getGenericHelper() *genericHelper
	compile(ctx context.Context, options *graphCompileOptions) (*composableRunnable, error)
	inputType() reflect.Type
	outputType() reflect.Type
	component() component
}
