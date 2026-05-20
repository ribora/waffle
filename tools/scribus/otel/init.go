// Package otel wires a scribus.Middleware that emits a SpanKindConsumer
// span and RED metrics around each delivery batch.
//
//	di.Include("gitlab.com/gorib/waffle/tools/otel")
//	di.Include("gitlab.com/gorib/waffle/tools/scribus")
//	di.Include("gitlab.com/gorib/waffle/tools/scribus/single") // or .../batch
//	di.Include("gitlab.com/gorib/waffle/tools/scribus/otel")
//
// The middleware bridges upstream trace context: gorib/amqp populates
// session.Traceparent from message headers, ExtractIntoContext lifts it to
// OTEL ctx, then the new span replaces it via MirrorIntoSession so the
// handler's downstream calls inherit this span as parent.
package otel

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/session"
	wotel "gitlab.com/gorib/waffle/tools/otel"
	"gitlab.com/gorib/waffle/tools/scribus"
	"gitlab.com/gorib/waffle/wiring/tracectx"
)

const instrumentationName = "gitlab.com/gorib/waffle/tools/scribus"

func InitScribusOtel() {
	di.Wire[scribus.Middleware](NewMiddleware, di.Tag(scribus.MiddlewareTag))
}

func NewMiddleware(tp wotel.TracerProvider, mp wotel.MeterProvider) *middleware {
	meter := mp.Meter(instrumentationName)
	duration, _ := meter.Float64Histogram(
		"messaging.consume.duration",
		metric.WithDescription("Duration of an AMQP delivery batch handler"),
		metric.WithUnit("s"),
	)
	messages, _ := meter.Int64Counter(
		"messaging.consume.messages",
		metric.WithDescription("Total AMQP messages consumed (counted per delivery in the batch)"),
	)
	return &middleware{
		tracer:   tp.Tracer(instrumentationName),
		duration: duration,
		messages: messages,
	}
}

type middleware struct {
	tracer   trace.Tracer
	duration metric.Float64Histogram
	messages metric.Int64Counter
}

func (m *middleware) Wrap(next scribus.HandlerFunc) scribus.HandlerFunc {
	return func(ctx context.Context, keys []string, documents [][]byte) error {
		ctx = tracectx.ExtractIntoContext(ctx)
		ctx, span := m.tracer.Start(ctx, "scribus.consume",
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.StringSlice("messaging.routing_keys", keys),
				attribute.Int("messaging.batch.size", len(keys)),
			),
		)
		defer span.End()
		tracectx.MirrorIntoSession(ctx, span.SpanContext())
		if s := session.FromContext(ctx); s != nil && s.CorrelationId != "" {
			span.SetAttributes(attribute.String("correlation_id", s.CorrelationId))
		}

		start := time.Now()
		err := next(ctx, keys, documents)
		took := time.Since(start)

		status := "ok"
		if err != nil {
			status = "error"
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
		}

		for _, key := range keys {
			m.messages.Add(ctx, 1, metric.WithAttributes(
				attribute.String("messaging.routing_key", key),
				attribute.String("status", status),
			))
		}
		m.duration.Record(ctx, took.Seconds(), metric.WithAttributes(
			attribute.String("status", status),
			attribute.String("batch_size", batchBucket(len(keys))),
		))

		return err
	}
}

// Bucketed for bounded cardinality — raw batch size as a label would explode.
func batchBucket(n int) string {
	switch {
	case n == 1:
		return "1"
	case n <= 10:
		return "2-10"
	case n <= 100:
		return "11-100"
	default:
		return ">100"
	}
}
