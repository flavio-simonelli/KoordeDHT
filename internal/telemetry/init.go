package telemetry

import (
	"KoordeDHT/internal/config"
	"KoordeDHT/internal/domain"
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func InitTracer(cfg config.TelemetryConfig, serviceName string, nodeId domain.ID) func(context.Context) error {
	if !cfg.Tracing.Enabled {
		log.Println("Tracing disabled")
		return func(context.Context) error { return nil }
	}

	attrs := append(
		[]attribute.KeyValue{
			semconv.ServiceNameKey.String(serviceName),
		},
		IdAttributes("dht.node.id", nodeId)...,
	)

	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(attrs...),
	)

	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
	}

	var tp *sdktrace.TracerProvider

	switch cfg.Tracing.Exporter {
	case "stdout":
		exp, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(res),
		)
	case "jaeger":
		exp, err := jaeger.New(
			jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.Tracing.Endpoint)),
		)
		if err != nil {
			log.Fatalf("failed to initialize Jaeger exporter: %v", err)
		}
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(res),
		)
	case "otlp":
		exp, err := otlptracegrpc.New(
			context.Background(),
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint(cfg.Tracing.Endpoint), // porta OTLP
		)
		if err != nil {
			log.Fatalf("failed to initialize OTLP exporter: %v", err)
		}
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(res),
		)
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

	return tp.Shutdown
}
