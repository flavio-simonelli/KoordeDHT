package trace

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/oklog/ulid/v2"
	"google.golang.org/grpc/metadata"
)

// GenerateTraceID crea un traceID globale univoco nel formato:
//
//	<nodeID>-<ULID>
func GenerateTraceID(nodeID string) string {
	t := time.Now().UTC()
	entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
	return fmt.Sprintf("%s-%s", nodeID, ulid.MustNew(ulid.Timestamp(t), entropy).String())
}

// AttachTraceID prende il traceID dal contesto o ne genera uno nuovo
func AttachTraceID(ctx context.Context, nodeID string) (context.Context, string) {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	// Se esiste già lo riutilizza
	if vals := md.Get("trace-id"); len(vals) > 0 {
		return ctx, vals[0]
	}
	// Altrimenti generane uno nuovo
	traceID := GenerateTraceID(nodeID)
	newCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("trace-id", traceID))
	return newCtx, traceID
}

// getTraceID estrae il trace-id dai metadata, oppure "" se assente
func GetTraceID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get("trace-id")
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}
