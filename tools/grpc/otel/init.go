// Package otel wires unary and stream server interceptors that emit
// server-side spans AND RED metrics per RPC, plus W3C trace-context
// propagation.
//
//	di.Include("gitlab.com/gorib/waffle/tools/grpc")
//	di.Include("gitlab.com/gorib/waffle/tools/otel")
//	di.Include("gitlab.com/gorib/waffle/tools/grpc/otel")
//
// otelgrpc upstream switched to stats.Handler-based instrumentation, which
// gitlab.com/gorib/grpc/server does not currently expose. We therefore use
// the global TextMapPropagator + injected TracerProvider directly.
package otel

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	baseGrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/session"
	wgrpc "gitlab.com/gorib/waffle/tools/grpc"
	wotel "gitlab.com/gorib/waffle/tools/otel"
	"gitlab.com/gorib/waffle/wiring/tracectx"
)

const instrumentationName = "gitlab.com/gorib/waffle/tools/grpc"

func InitGrpcOtel() {
	di.Wire[wgrpc.Middleware](NewUnaryMiddleware, di.Tag(wgrpc.MiddlewareTag))
	di.Wire[wgrpc.StreamMiddleware](NewStreamMiddleware, di.Tag(wgrpc.StreamMiddlewareTag))
}

type instruments struct {
	tracer   trace.Tracer
	duration metric.Float64Histogram
	requests metric.Int64Counter
}

func buildInstruments(tp wotel.TracerProvider, mp wotel.MeterProvider) instruments {
	meter := mp.Meter(instrumentationName)
	duration, _ := meter.Float64Histogram(
		"rpc.server.duration",
		metric.WithDescription("Duration of gRPC server-side request handling"),
		metric.WithUnit("s"),
	)
	requests, _ := meter.Int64Counter(
		"rpc.server.requests",
		metric.WithDescription("Total gRPC server-side requests"),
	)
	return instruments{
		tracer:   tp.Tracer(instrumentationName),
		duration: duration,
		requests: requests,
	}
}

func NewUnaryMiddleware(tp wotel.TracerProvider, mp wotel.MeterProvider) *unaryMiddleware {
	return &unaryMiddleware{instruments: buildInstruments(tp, mp)}
}

type unaryMiddleware struct {
	instruments
}

func (m *unaryMiddleware) Run() baseGrpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *baseGrpc.UnaryServerInfo, handler baseGrpc.UnaryHandler) (any, error) {
		ctx, span := startServerSpan(ctx, m.tracer, info.FullMethod)
		defer span.End()
		start := time.Now()
		resp, err := handler(ctx, req)
		code := recordRPCStatus(span, err)
		m.record(ctx, info.FullMethod, code, time.Since(start))
		return resp, err
	}
}

func NewStreamMiddleware(tp wotel.TracerProvider, mp wotel.MeterProvider) *streamMiddleware {
	return &streamMiddleware{instruments: buildInstruments(tp, mp)}
}

type streamMiddleware struct {
	instruments
}

func (m *streamMiddleware) Run() baseGrpc.StreamServerInterceptor {
	return func(srv any, ss baseGrpc.ServerStream, info *baseGrpc.StreamServerInfo, handler baseGrpc.StreamHandler) error {
		ctx, span := startServerSpan(ss.Context(), m.tracer, info.FullMethod)
		defer span.End()
		start := time.Now()
		err := handler(srv, &wrappedStream{ServerStream: ss, ctx: ctx})
		code := recordRPCStatus(span, err)
		m.record(ctx, info.FullMethod, code, time.Since(start))
		return err
	}
}

func (i instruments) record(ctx context.Context, fullMethod string, code codes.Code, took time.Duration) {
	service, method := splitFullMethod(fullMethod)
	attrs := metric.WithAttributes(
		attribute.String("rpc.system", "grpc"),
		attribute.String("rpc.service", service),
		attribute.String("rpc.method", method),
		attribute.Int("rpc.grpc.status_code", int(code)),
	)
	i.duration.Record(ctx, took.Seconds(), attrs)
	i.requests.Add(ctx, 1, attrs)
}

// "/pkg.Service/Method" → ("pkg.Service", "Method"). Bounded cardinality.
func splitFullMethod(full string) (service, method string) {
	full = strings.TrimPrefix(full, "/")
	if i := strings.LastIndex(full, "/"); i >= 0 {
		return full[:i], full[i+1:]
	}
	return full, ""
}

func startServerSpan(ctx context.Context, tracer trace.Tracer, method string) (context.Context, trace.Span) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		ctx = otel.GetTextMapPropagator().Extract(ctx, metadataCarrier(md))
	}
	ctx, span := tracer.Start(ctx, method,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attribute.String("rpc.system", "grpc")),
	)
	if s := session.FromContext(ctx); s != nil {
		if s.CorrelationId != "" {
			span.SetAttributes(attribute.String("correlation_id", s.CorrelationId))
		}
		tracectx.MirrorIntoSession(ctx, span.SpanContext())
	}
	return ctx, span
}

func recordRPCStatus(span trace.Span, err error) codes.Code {
	code := codes.OK
	if err != nil {
		if st, ok := status.FromError(err); ok {
			code = st.Code()
		} else {
			code = codes.Unknown
		}
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
	}
	span.SetAttributes(attribute.Int("rpc.grpc.status_code", int(code)))
	return code
}

type wrappedStream struct {
	baseGrpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }

type metadataCarrier metadata.MD

func (m metadataCarrier) Get(key string) string {
	vs := metadata.MD(m).Get(key)
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

func (m metadataCarrier) Set(key, value string) { metadata.MD(m).Set(key, value) }
func (m metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

var _ propagation.TextMapCarrier = metadataCarrier{}
