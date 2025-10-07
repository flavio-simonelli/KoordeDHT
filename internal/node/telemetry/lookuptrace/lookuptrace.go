package lookuptrace

import (
	"context"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	lookupMetaKey = "x-koorde-lookup"
	tracerName    = "koorde/lookuptrace"
)

var tracer = otel.Tracer(tracerName)

// WithLookup adds the flag to the output metadata.
func WithLookup(ctx context.Context) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	md.Set(lookupMetaKey, "true")
	return metadata.NewOutgoingContext(ctx, md)
}

// IsLookup checks whether the incoming context belongs to a lookup.
func IsLookup(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	values := md.Get(lookupMetaKey)
	return len(values) > 0 && values[0] == "true"
}

// ServerInterceptor creates spans only for marked Lookup and FindSuccessor
// and propagates the OTEL context with hop count and lookup flag
func ServerInterceptor() grpc.UnaryServerInterceptor {
	propagator := otel.GetTextMapPropagator()

	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {

		method := info.FullMethod

		// Only trace Lookup and FindSuccessor calls
		// (FindSuccessor only if it's part of a lookup)
		if strings.Contains(method, "Lookup") || (strings.Contains(method, "FindSuccessor") && IsLookup(ctx)) {
			ctx = WithLookup(ctx)

			// Increment hop count from metadata
			var hopCount int
			if md, ok := metadata.FromIncomingContext(ctx); ok {
				if vals := md.Get("x-koorde-hop"); len(vals) > 0 {
					hopCount, _ = strconv.Atoi(vals[0])
				}

				// Extract OTEL context from metadata
				ctx = propagator.Extract(ctx, metadataCarrier(md))
			}

			// Create new metadata with incremented hop count
			ctx, span := tracer.Start(ctx, method, trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()

			// Publish attributes to the span
			span.SetAttributes(
				attribute.String("rpc.method", method),
				attribute.Int("koorde.hop", hopCount),
			)

			// Execute the real handler
			return handler(ctx, req)
		}

		// Not a lookup-related call, proceed without tracing
		return handler(ctx, req)
	}
}

// ClientInterceptor creates spans only for marked Lookup and FindSuccessor
func ClientInterceptor() grpc.UnaryClientInterceptor {
	propagator := otel.GetTextMapPropagator()

	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if IsLookup(ctx) {
			ctx = WithLookup(ctx)

			// Increment hop count from metadata
			var hopCount int
			if md, ok := metadata.FromOutgoingContext(ctx); ok {
				if vals := md.Get("x-koorde-hop"); len(vals) > 0 {
					hopCount, _ = strconv.Atoi(vals[0])
				}
			}
			hopCount++

			md, _ := metadata.FromOutgoingContext(ctx)
			md = md.Copy()
			md.Set("x-koorde-hop", strconv.Itoa(hopCount))

			// Create new outgoing context with updated metadata
			ctx = metadata.NewOutgoingContext(ctx, md)
			ctx, span := tracer.Start(ctx, method, trace.WithSpanKind(trace.SpanKindClient))
			defer span.End()

			// Publish attributes to the span
			span.SetAttributes(attribute.Int("koorde.hop", hopCount))

			// Inject the span context into the metadata
			propagator.Inject(ctx, metadataCarrier(md))
			ctx = metadata.NewOutgoingContext(ctx, md)

			return invoker(ctx, method, req, reply, cc, opts...)
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

type metadataCarrier metadata.MD

func (mc metadataCarrier) Get(key string) string {
	vals := metadata.MD(mc).Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (mc metadataCarrier) Set(key, value string) {
	metadata.MD(mc).Set(key, value)
}

func (mc metadataCarrier) Keys() []string {
	out := make([]string, 0, len(mc))
	for k := range mc {
		out = append(out, k)
	}
	return out
}
