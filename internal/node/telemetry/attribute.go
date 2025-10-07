package telemetry

import (
	"KoordeDHT/internal/domain"

	"go.opentelemetry.io/otel/attribute"
)

func IdAttributes(prefix string, id domain.ID) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(prefix+".dec", id.ToBigInt().String()),
		attribute.String(prefix+".hex", id.ToHexString(true)),
		attribute.String(prefix+".bin", id.ToBinaryString(true)),
	}
}
