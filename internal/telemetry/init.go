package telemetry

import (
	"KoordeDHT/internal/config"
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func InitTracer(cfg config.TelemetryConfig, serviceName string) func(context.Context) error {
	if !cfg.Tracing.Enabled {
		log.Println("Tracing disabled")
		return func(context.Context) error { return nil }
	}

	var (
		tp *sdktrace.TracerProvider
	)

	switch cfg.Tracing.Exporter {
	case "stdout":
		exp, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(serviceName),
			)),
		)
	case "jaeger":
		exp, err := jaeger.New(
			jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://localhost:14268/api/traces")),
		)
		if err != nil {
			log.Fatalf("failed to initialize Jaeger exporter: %v", err)
		}
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(serviceName),
			)),
		)
	// TODO: aggiungere "otlp", "zipkin", ecc.
	default:
		panic(fmt.Sprintf("unsupported exporter: %s", cfg.Tracing.Exporter))
	}

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, // traceparent/tracestate W3C
			propagation.Baggage{},      // key=value opzionali
		),
	)

	otel.SetTracerProvider(tp)
	return tp.Shutdown
}
