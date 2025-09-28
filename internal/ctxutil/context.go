package ctxutil

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CheckContext verifies whether the provided context has been canceled
// or its deadline has expired.
//
// Behavior:
//   - If ctx.Err() == context.Canceled, returns a gRPC error with code Canceled.
//   - If ctx.Err() == context.DeadlineExceeded, returns a gRPC error with code DeadlineExceeded.
//   - Otherwise (ctx.Err() == nil), returns nil.
//
// This helper is typically invoked at the beginning of an RPC handler
// to ensure that the request is still valid before performing any work.
func CheckContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			return status.Error(codes.Canceled, "request canceled")
		case errors.Is(err, context.DeadlineExceeded):
			return status.Error(codes.DeadlineExceeded, "deadline exceeded")
		default:
			// In practice, ctx.Err() is only one of the above,
			// but we keep default for forward-compatibility.
			return status.Error(codes.Unknown, err.Error())
		}
	}
	return nil
}
