// Package otel wires a fiber middleware that emits server-side spans AND
// RED metrics (rate / errors / duration) per HTTP request, and mirrors the
// span traceparent into session for outgoing-call propagation.
//
//	di.Include("gitlab.com/gorib/waffle/tools/otel")
//	di.Include("gitlab.com/gorib/waffle/tools/http")
//	di.Include("gitlab.com/gorib/waffle/tools/http/otel")
//
// Auto-mounted via the Globaler interface at Order=350 — between session
// (300) and log (400). Nothing to add in Router.Setup.
package otel

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/session"
	whttp "gitlab.com/gorib/waffle/tools/http"
	wotel "gitlab.com/gorib/waffle/tools/otel"
	"gitlab.com/gorib/waffle/wiring/tracectx"
)

// Name — middleware identifier. Auto-mount makes manual lookup unnecessary,
// but the constant is exported for code that opts out and applies it by name.
const Name = "otel"

const instrumentationName = "gitlab.com/gorib/waffle/tools/http"

func InitHttpOtel() {
	di.Wire[whttp.Middleware](NewMiddleware, di.Tag(whttp.MiddlewareTag))
}

func NewMiddleware(tp wotel.TracerProvider, mp wotel.MeterProvider) *middleware {
	meter := mp.Meter(instrumentationName)
	requestDuration, _ := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP server requests"),
		metric.WithUnit("s"),
	)
	activeRequests, _ := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of HTTP requests currently being served"),
	)
	requestsTotal, _ := meter.Int64Counter(
		"http.server.requests",
		metric.WithDescription("Total HTTP server requests"),
	)
	return &middleware{
		tracer:          tp.Tracer(instrumentationName),
		requestDuration: requestDuration,
		activeRequests:  activeRequests,
		requestsTotal:   requestsTotal,
	}
}

type middleware struct {
	tracer          trace.Tracer
	requestDuration metric.Float64Histogram
	activeRequests  metric.Int64UpDownCounter
	requestsTotal   metric.Int64Counter
}

func (middleware) Name() string    { return Name }
func (*middleware) IsGlobal() bool { return true }
func (*middleware) Order() int     { return 350 }

func (m *middleware) Run() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := otel.GetTextMapPropagator().Extract(c.UserContext(), headerCarrier{c: c})

		ctx, span := m.tracer.Start(ctx, c.Path(),
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.method", c.Method()),
				attribute.String("http.target", c.OriginalURL()),
				attribute.String("http.scheme", c.Protocol()),
				attribute.String("http.user_agent", string(c.Request().Header.UserAgent())),
			),
		)
		defer span.End()
		c.SetUserContext(ctx)

		if s := session.FromContext(ctx); s != nil {
			if s.CorrelationId != "" {
				span.SetAttributes(attribute.String("correlation_id", s.CorrelationId))
			}
			tracectx.MirrorIntoSession(ctx, span.SpanContext())
		}

		m.activeRequests.Add(ctx, 1)
		defer m.activeRequests.Add(ctx, -1)
		start := time.Now()

		err := c.Next()

		status := c.Response().StatusCode()
		span.SetAttributes(attribute.Int("http.status_code", status))
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else if status >= 500 {
			span.SetStatus(codes.Error, "status "+strconv.Itoa(status))
		}

		// route() = matched template (e.g. "/users/:id"), bounded — not raw URL.
		attrs := metric.WithAttributes(
			attribute.String("http.method", c.Method()),
			attribute.String("http.route", route(c)),
			attribute.Int("http.status_code", status),
		)
		m.requestDuration.Record(ctx, time.Since(start).Seconds(), attrs)
		m.requestsTotal.Add(ctx, 1, attrs)

		return err
	}
}

func route(c *fiber.Ctx) string {
	if r := c.Route(); r != nil {
		return r.Path
	}
	return ""
}

type headerCarrier struct{ c *fiber.Ctx }

func (h headerCarrier) Get(key string) string { return string(h.c.Request().Header.Peek(key)) }
func (h headerCarrier) Set(key, value string) { h.c.Request().Header.Set(key, value) }
func (h headerCarrier) Keys() []string {
	keys := make([]string, 0)
	h.c.Request().Header.VisitAll(func(k, _ []byte) { keys = append(keys, string(k)) })
	return keys
}

var _ propagation.TextMapCarrier = headerCarrier{}
