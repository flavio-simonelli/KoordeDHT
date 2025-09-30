package lookuptrace

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	lookupMetaKey = "x-koorde-lookup"
	tracerName    = "koorde/lookuptrace"
)

var tracer = otel.Tracer(tracerName)

// WithLookup aggiunge il flag nei metadata in uscita.
func WithLookup(ctx context.Context) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	md.Set(lookupMetaKey, "true")
	return metadata.NewOutgoingContext(ctx, md)
}

// IsLookup controlla se il contesto in ingresso appartiene a una lookup.
func IsLookup(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	values := md.Get(lookupMetaKey)
	return len(values) > 0 && values[0] == "true"
}

// ServerInterceptor crea span solo per Lookup e FindSuccessor marcati.
func ServerInterceptor() grpc.UnaryServerInterceptor {
	propagator := otel.GetTextMapPropagator()

	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// estrai contesto OTEL dai metadata in ingresso
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			ctx = propagator.Extract(ctx, metadataCarrier(md))
		}

		method := info.FullMethod

		if strings.Contains(method, "Lookup") {
			ctx = WithLookup(ctx)
			ctx, span := tracer.Start(ctx, method, trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()
			return handler(ctx, req)
		}

		if strings.Contains(method, "FindSuccessor") && IsLookup(ctx) {
			ctx = WithLookup(ctx)
			ctx, span := tracer.Start(ctx, method, trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()
			return handler(ctx, req)
		}

		return handler(ctx, req)
	}
}

// ClientInterceptor propaga il flag lookup e crea span client-side.
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
			ctx, span := tracer.Start(ctx, method, trace.WithSpanKind(trace.SpanKindClient))
			defer span.End()

			// inietta contesto OTEL nei metadata
			md, _ := metadata.FromOutgoingContext(ctx)
			md = md.Copy()
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
