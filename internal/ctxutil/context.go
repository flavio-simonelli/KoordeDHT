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
//   - If ctx.Err() == context.Canceled, it returns a gRPC error with code Canceled.
//   - If ctx.Err() == context.DeadlineExceeded, it returns a gRPC error with code DeadlineExceeded.
//   - Otherwise, it returns nil, meaning the context is still active.
//
// This helper is typically invoked at the beginning of an RPC handler
// to ensure that the request is still valid before performing any work.
func CheckContext(ctx context.Context) error {
	switch err := ctx.Err(); {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request was canceled by client")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return nil
	}
}
