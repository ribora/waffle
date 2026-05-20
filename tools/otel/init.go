// Package otel wires the OpenTelemetry SDK (TracerProvider, MeterProvider,
// global propagator) into the DI container.
//
// Activation is opt-in via OTLP standard env-vars:
//
//	OTEL_EXPORTER_OTLP_ENDPOINT          general OTLP target
//	OTEL_EXPORTER_OTLP_TRACES_ENDPOINT   per-signal override (optional)
//	OTEL_EXPORTER_OTLP_METRICS_ENDPOINT  per-signal override (optional)
//	OTEL_SERVICE_NAME                    service.name resource attribute
//	OTEL_RESOURCE_ATTRIBUTES             key=value,key=value
//	OTEL_TRACES_SAMPLER                  always_on (default) | always_off | parentbased_traceidratio | traceidratio
//	OTEL_TRACES_SAMPLER_ARG              numeric argument for ratio samplers
//
// If no endpoint is configured, both providers fall back to noop
// implementations so that every tracer.Start / counter.Add in instrumented
// code becomes a near-zero-cost no-op. No `if otelEnabled` branches are
// needed downstream.
package otel

import (
	"context"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"gitlab.com/gorib/di"
)

// TracerProvider embeds io.Closer so diogen registers Shutdown in the
// cleanup chain, flushing pending spans on process exit.
type TracerProvider interface {
	trace.TracerProvider
	Close() error
}

// MeterProvider — same Close→Shutdown rationale as TracerProvider.
type MeterProvider interface {
	metric.MeterProvider
	Close() error
}

func InitOtel() {
	di.Wire[TracerProvider](NewTracerProvider)
	di.Wire[MeterProvider](NewMeterProvider)
	di.Define(NewPropagator)
}

func NewTracerProvider() (TracerProvider, error) {
	endpoint := tracesEndpoint()
	if endpoint == "" {
		return noopTracer{TracerProvider: tracenoop.NewTracerProvider()}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	res, err := buildResource(ctx)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(buildSampler()),
	)
	otel.SetTracerProvider(tp)
	return sdkTracer{TracerProvider: tp}, nil
}

func NewMeterProvider() (MeterProvider, error) {
	endpoint := metricsEndpoint()
	if endpoint == "" {
		return noopMeter{MeterProvider: metricnoop.NewMeterProvider()}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpoint(endpoint), otlpmetricgrpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	res, err := buildResource(ctx)
	if err != nil {
		return nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)
	return sdkMeter{MeterProvider: mp}, nil
}

// NewPropagator installs the result as the otel global so library code that
// calls otel.GetTextMapPropagator() sees it.
func NewPropagator() propagation.TextMapPropagator {
	p := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(p)
	return p
}

func tracesEndpoint() string {
	if v := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"); v != "" {
		return v
	}
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
}

func metricsEndpoint() string {
	if v := os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"); v != "" {
		return v
	}
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
}

func buildResource(ctx context.Context) (*resource.Resource, error) {
	attrs := []resource.Option{resource.WithFromEnv(), resource.WithTelemetrySDK()}
	if name := os.Getenv("OTEL_SERVICE_NAME"); name != "" {
		attrs = append(attrs, resource.WithAttributes(semconv.ServiceName(name)))
	}
	return resource.New(ctx, attrs...)
}

func buildSampler() sdktrace.Sampler {
	name := os.Getenv("OTEL_TRACES_SAMPLER")
	arg := os.Getenv("OTEL_TRACES_SAMPLER_ARG")
	switch name {
	case "", "always_on", "parentbased_always_on":
		return sdktrace.AlwaysSample()
	case "always_off", "parentbased_always_off":
		return sdktrace.NeverSample()
	case "traceidratio":
		return sdktrace.TraceIDRatioBased(parseRatio(arg, 1.0))
	case "parentbased_traceidratio":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(parseRatio(arg, 1.0)))
	default:
		return sdktrace.AlwaysSample()
	}
}

func parseRatio(s string, def float64) float64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}

type sdkTracer struct{ *sdktrace.TracerProvider }

func (s sdkTracer) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.TracerProvider.Shutdown(ctx)
}

type sdkMeter struct{ *sdkmetric.MeterProvider }

func (s sdkMeter) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.MeterProvider.Shutdown(ctx)
}

type noopTracer struct{ trace.TracerProvider }

func (noopTracer) Close() error { return nil }

type noopMeter struct{ metric.MeterProvider }

func (noopMeter) Close() error { return nil }
