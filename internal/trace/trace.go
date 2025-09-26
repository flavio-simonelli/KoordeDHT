package trace

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"KoordeDHT/internal/domain"

	"github.com/oklog/ulid/v2"
)

type traceKey struct{}

// GenerateTraceID crea un traceID globale univoco nel formato:
//
//	<nodeID>-<ULID>
func GenerateTraceID(nodeID string) string {
	t := time.Now().UTC()
	entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
	id := ulid.MustNew(ulid.Timestamp(t), entropy)
	return fmt.Sprintf("%s-%s", nodeID, id.String())
}

// AttachTraceID genera e inserisce un traceID nel contesto
// a partire dal nodeID fornito. Restituisce il nuovo contesto e il traceID.
func AttachTraceID(ctx context.Context, nodeID domain.ID) (context.Context, string) {
	traceID := GenerateTraceID(nodeID.String())
	return context.WithValue(ctx, traceKey{}, traceID), traceID
}

// GetTraceID recupera il traceID dal contesto.
// Se non è presente ritorna "".
func GetTraceID(ctx context.Context) string {
	if v := ctx.Value(traceKey{}); v != nil {
		if id, ok := v.(string); ok && id != "" {
			return id
		}
	}
	return ""
}
