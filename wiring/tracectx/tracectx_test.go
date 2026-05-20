package tracectx

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"gitlab.com/gorib/session"
)

func TestFormatTraceparent_RoundTrip(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	spanID, _ := trace.SpanIDFromHex("b7ad6b7169203331")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	got := FormatTraceparent(sc)
	want := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}

	tid, sid, flags, ok := parseTraceparent(got)
	if !ok {
		t.Fatal("parseTraceparent returned !ok on its own output")
	}
	if tid != traceID || sid != spanID || flags != trace.FlagsSampled {
		t.Errorf("round-trip mismatch: tid=%v sid=%v flags=%v", tid, sid, flags)
	}
}

func TestParseTraceparent_Invalid(t *testing.T) {
	cases := []string{
		"",
		"00-tooshort",
		"00-0af7651916cd43dd8448eb211c80319c_b7ad6b7169203331-01", // wrong separator
		"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-0X", // bad flags hex
		"00-XX7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",  // bad traceID hex
	}
	for _, tp := range cases {
		if _, _, _, ok := parseTraceparent(tp); ok {
			t.Errorf("parseTraceparent(%q) = ok, want false", tp)
		}
	}
}

func TestExtractIntoContext_NoSession(t *testing.T) {
	ctx := ExtractIntoContext(context.Background())
	if trace.SpanContextFromContext(ctx).IsValid() {
		t.Error("expected no span context when session is missing")
	}
}

func TestExtractIntoContext_MalformedTraceparent(t *testing.T) {
	ctx := session.NewContextWithSession(context.Background(), session.Session{
		Traceparent: "not-a-valid-traceparent",
	})
	out := ExtractIntoContext(ctx)
	if trace.SpanContextFromContext(out).IsValid() {
		t.Error("malformed traceparent should not produce valid SpanContext")
	}
}

func TestExtractIntoContext_Valid(t *testing.T) {
	ctx := session.NewContextWithSession(context.Background(), session.Session{
		Traceparent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	})
	out := ExtractIntoContext(ctx)
	sc := trace.SpanContextFromContext(out)
	if !sc.IsValid() {
		t.Fatal("expected valid SpanContext")
	}
	if !sc.IsRemote() {
		t.Error("expected Remote=true")
	}
	if sc.TraceID().String() != "0af7651916cd43dd8448eb211c80319c" {
		t.Errorf("trace id = %s", sc.TraceID().String())
	}
}

func TestMirrorIntoSession_NoSession(t *testing.T) {
	// Should not panic.
	traceID, _ := trace.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	spanID, _ := trace.SpanIDFromHex("b7ad6b7169203331")
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID})
	MirrorIntoSession(context.Background(), sc)
}

func TestMirrorIntoSession_Writes(t *testing.T) {
	ctx := session.NewContextWithSession(context.Background(), session.Session{})
	traceID, _ := trace.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	spanID, _ := trace.SpanIDFromHex("b7ad6b7169203331")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	MirrorIntoSession(ctx, sc)
	s := session.FromContext(ctx)
	if s == nil {
		t.Fatal("session missing")
	}
	want := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	if s.Traceparent != want {
		t.Errorf("traceparent = %q, want %q", s.Traceparent, want)
	}
}

func TestForkCtx_Decouples(t *testing.T) {
	ctx := session.NewContextWithSession(context.Background(), session.Session{
		CorrelationId: "abc",
		Traceparent:   "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	})
	forked := ForkCtx(ctx)

	parent := session.FromContext(ctx)
	child := session.FromContext(forked)
	if parent == child {
		t.Fatal("ForkCtx returned the same *Session pointer")
	}
	if child.CorrelationId != "abc" || child.Traceparent != parent.Traceparent {
		t.Error("forked session did not preserve fields")
	}

	// mutate child, parent must stay
	child.Traceparent = "different"
	if parent.Traceparent == child.Traceparent {
		t.Error("mutation of forked session leaked into parent")
	}
}

func TestForkCtx_NoSession(t *testing.T) {
	ctx := context.Background()
	out := ForkCtx(ctx)
	if out != ctx {
		t.Error("ForkCtx with no session should return ctx unchanged")
	}
}
