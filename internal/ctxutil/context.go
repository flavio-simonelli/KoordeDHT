package ctxutil

import (
	"context"
	"errors"
	"time"

	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/trace"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// unexported keys to avoid collisions
type traceKey struct{}
type hopsKey struct{}

// ContextOption configures the behavior of NewContext.
// Multiple options can be combined.
type ContextOption func(*ctxConfig)

type ctxConfig struct {
	withTrace bool
	withHops  bool
	nodeID    domain.ID
	timeout   time.Duration
}

// WithTrace enables attaching a fresh traceID to the created context.
// The traceID is derived from the provided nodeID and returned by NewContext.
func WithTrace(nodeID domain.ID) ContextOption {
	return func(cfg *ctxConfig) {
		cfg.withTrace = true
		cfg.nodeID = nodeID
	}
}

// WithTimeout sets a timeout duration for the created context.
// The caller must defer the cancel function returned by NewContext.
func WithTimeout(d time.Duration) ContextOption {
	return func(cfg *ctxConfig) {
		cfg.timeout = d
	}
}

// WithHops initializes the hop counter at 0 in the context.
func WithHops() ContextOption {
	return func(cfg *ctxConfig) {
		cfg.withHops = true
	}
}

// NewContext creates a new context configured according to the provided options.
//
// Options:
//   - WithTrace(nodeID): attaches a traceID to the context
//   - WithTimeout(d): applies a timeout to the context
//
// Returns:
//   - context.Context: the configured context
//   - context.CancelFunc: a cancel function (nil if no timeout was set)
func NewContext(opts ...ContextOption) (context.Context, context.CancelFunc) {
	cfg := &ctxConfig{}
	for _, o := range opts {
		o(cfg)
	}

	// base context
	var ctx context.Context
	var cancel context.CancelFunc
	if cfg.timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), cfg.timeout)
	} else {
		ctx = context.Background()
	}
	if cfg.withTrace {
		ctx, _ = trace.AttachTraceID(ctx, cfg.nodeID)
	}
	if cfg.withHops {
		ctx = context.WithValue(ctx, hopsKey{}, 0)
	}

	return ctx, cancel
}

// TraceIDFromContext extracts the traceID from metadata.
// Returns an empty string if not present.
func TraceIDFromContext(ctx context.Context) string {
	return trace.GetTraceID(ctx)
}

// EnsureTraceID checks if the context already has a non-empty traceID.
// If not, it attaches a new one derived from the provided nodeID.
// Returns the updated context (may be the same as input).
func EnsureTraceID(ctx context.Context, nodeID domain.ID) context.Context {
	if id := trace.GetTraceID(ctx); id == "" {
		ctx, _ = trace.AttachTraceID(ctx, nodeID)
	}
	return ctx
}

// HopsFromContext returns the current hop counter from the context.
// If not present, it returns -1 to indicate "not set".
func HopsFromContext(ctx context.Context) int {
	val := ctx.Value(hopsKey{})
	if hops, ok := val.(int); ok {
		return hops
	}
	return -1
}

// IncHops increments the hop counter in the context if present.
// If no hop counter is set, the original context is returned unchanged.
// Special case: if the hop counter is -1, it remains -1.
func IncHops(ctx context.Context) context.Context {
	val := ctx.Value(hopsKey{})
	if hops, ok := val.(int); ok {
		if hops == -1 {
			// -1 significa "non conteggiare", non incrementare
			return ctx
		}
		return context.WithValue(ctx, hopsKey{}, hops+1)
	}
	return ctx
}

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
