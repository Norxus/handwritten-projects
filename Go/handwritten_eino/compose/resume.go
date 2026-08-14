package compose

import (
	"context"

	"github.com/cloudwego/eino/internal/core"
)

func GetInterruptState[T any](ctx context.Context)(wasInterrupted bool, hasState bool, state T) {
	return core.GetInterruptState[T](ctx)
}

func AppendAddressSegment(ctx context.Context, segType AddressSegmentType, segID string) context.Context {
	return core.AppendAddressSegment(ctx, segType, segID, "")
}

func appendToolAddressSegment(ctx context.Context, segID string, subID string)  context.Context {
	return core.AppendAddressSegment(ctx, AddressSegmentTool, segID, subID)
}