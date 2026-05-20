// Package tracectx bridges session.Traceparent and OpenTelemetry's
// context-bound active span.
//
// Why this exists:
//   - session.Session carries Traceparent/Tracestate strings that flow across
//     HTTP/gRPC/AMQP boundaries (set by transport-layer middleware in
//     gorib/http, gorib/grpc, gorib/amqp).
//   - OTEL spans live on the OTEL ctx, not in session.
//
// Bridging is needed in two directions:
//
//	StartSpan(ctx, name) — start an OTEL span AND mirror the new
//	    traceparent into session.Traceparent so that subsequent outgoing
//	    HTTP/gRPC/AMQP calls (which read from session) propagate the
//	    current span as the parent.
//
//	ExtractIntoContext(ctx) — read session.Traceparent and inject it as
//	    the parent SpanContext into the OTEL ctx. Used at consume-side
//	    entry points (e.g. AMQP handlers) where session is filled but
//	    OTEL doesn't yet know about the upstream trace.
//
// Concurrency note: mirroring writes to *session.Session. If you fan out
// into goroutines that each start their own span on the SAME ctx, those
// writes race. Use ForkCtx to give each goroutine its own Session.
package tracectx

import (
	"context"
	"encoding/hex"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"gitlab.com/gorib/session"
)

// StartSpan starts a span on the default global TracerProvider and updates
// session.Traceparent in-place so client-side propagation forwards the new
// span as the parent. Returns the same shape as tracer.Start.
func StartSpan(ctx context.Context, tracerName, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, spanName, opts...)
	MirrorIntoSession(ctx, span.SpanContext())
	return ctx, span
}

// ExtractIntoContext reads session.Traceparent/Tracestate (e.g. just
// populated by an AMQP consumer) and installs the parsed SpanContext into
// ctx so that subsequent OTEL span starts treat it as the parent.
//
// Returns ctx unchanged when session is missing or traceparent malformed.
func ExtractIntoContext(ctx context.Context) context.Context {
	s := session.FromContext(ctx)
	if s == nil || s.Traceparent == "" {
		return ctx
	}
	traceID, spanID, flags, ok := parseTraceparent(s.Traceparent)
	if !ok {
		return ctx
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: flags,
		Remote:     true,
	})
	return trace.ContextWithRemoteSpanContext(ctx, sc)
}

// ForkCtx returns a ctx with a fresh copy of the current Session, so a
// goroutine can start its own span without racing the parent's Traceparent.
//
//	for _, item := range items {
//	    item := item
//	    g.Go(func() error {
//	        ctx := tracectx.ForkCtx(ctx)
//	        ctx, span := tracectx.StartSpan(ctx, "worker", "process")
//	        defer span.End()
//	        return process(ctx, item)
//	    })
//	}
func ForkCtx(ctx context.Context) context.Context {
	s := session.FromContext(ctx)
	if s == nil {
		return ctx
	}
	clone := *s
	return session.NewContextWithSession(ctx, clone)
}

// MirrorIntoSession copies a SpanContext into the Session attached to ctx.
// No-op when Session missing or span invalid.
func MirrorIntoSession(ctx context.Context, sc trace.SpanContext) {
	if !sc.IsValid() {
		return
	}
	s := session.FromContext(ctx)
	if s == nil {
		return
	}
	s.Traceparent = FormatTraceparent(sc)
	if ts := sc.TraceState().String(); ts != "" {
		s.Tracestate = ts
	}
}

// FormatTraceparent encodes a SpanContext as a W3C traceparent header value.
func FormatTraceparent(sc trace.SpanContext) string {
	traceID := sc.TraceID()
	spanID := sc.SpanID()
	flags := sc.TraceFlags()
	buf := make([]byte, 0, 55)
	buf = append(buf, "00-"...)
	buf = append(buf, hex.EncodeToString(traceID[:])...)
	buf = append(buf, '-')
	buf = append(buf, hex.EncodeToString(spanID[:])...)
	buf = append(buf, '-')
	buf = append(buf, hex.EncodeToString([]byte{byte(flags)})...)
	return string(buf)
}

func parseTraceparent(tp string) (trace.TraceID, trace.SpanID, trace.TraceFlags, bool) {
	var tid trace.TraceID
	var sid trace.SpanID
	var flags trace.TraceFlags
	if len(tp) < 55 || tp[2] != '-' || tp[35] != '-' || tp[52] != '-' {
		return tid, sid, flags, false
	}
	if _, err := hex.Decode(tid[:], []byte(tp[3:35])); err != nil {
		return tid, sid, flags, false
	}
	if _, err := hex.Decode(sid[:], []byte(tp[36:52])); err != nil {
		return tid, sid, flags, false
	}
	var fb [1]byte
	if _, err := hex.Decode(fb[:], []byte(tp[53:55])); err != nil {
		return tid, sid, flags, false
	}
	flags = trace.TraceFlags(fb[0])
	return tid, sid, flags, true
}
