# OpenTelemetry

Waffle ships an opt-in OpenTelemetry stack. When no OTLP endpoint is configured, the providers fall back to no-op implementations and instrumented code (spans, counters) costs effectively nothing. When you set `OTEL_EXPORTER_OTLP_ENDPOINT`, the same code starts emitting OTLP gRPC to a Collector.

## Setup

Three `di.Include` lines enable the full stack on an HTTP+gRPC service:

```go
func InitDi() {
    di.Include("gitlab.com/gorib/waffle/tools/otel")        // providers + propagator
    di.Include("gitlab.com/gorib/waffle/tools/http")
    di.Include("gitlab.com/gorib/waffle/tools/http/otel")   // otelfiber middleware
    di.Include("gitlab.com/gorib/waffle/tools/grpc")
    di.Include("gitlab.com/gorib/waffle/tools/grpc/otel")   // server interceptors
}
```

Each OTEL subpackage depends on `tools/otel` — including a subpackage without it will fail at `make generate` with `dependency for TracerProvider not found`. This is the intended opt-in safeguard.

## Configuration (OTLP standard env-vars)

Waffle does not invent its own variable names. The SDK reads:

| Variable | Purpose |
|----------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP/gRPC collector endpoint (e.g. `otel-collector:4317`). Empty → no-op providers. |
| `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` | Per-signal override for traces |
| `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` | Per-signal override for metrics |
| `OTEL_SERVICE_NAME` | `service.name` resource attribute |
| `OTEL_RESOURCE_ATTRIBUTES` | Additional `key=value,key=value` resource attributes |
| `OTEL_TRACES_SAMPLER` | `always_on` (default), `always_off`, `traceidratio`, `parentbased_traceidratio` |
| `OTEL_TRACES_SAMPLER_ARG` | Ratio for the two ratio samplers |

The providers implement `io.Closer`, so diogen wires `Shutdown` into the generated cleanup chain. Pending spans flush on graceful shutdown.

## What gets instrumented

| Tool | Activation | What's traced |
|------|------------|---------------|
| `tools/http/otel` | auto — wired as a `Globaler` middleware at Order=350 (between session and log) | Every fiber request → server span. http.method/route/status_code are attributes. |
| `tools/grpc/otel` | auto — wired as gRPC server interceptors | Every unary and streaming RPC → server span. Method name is the span name. gRPC status code captured. |
| `tools/scribus/otel` | auto — wired as scribus middleware | Every AMQP delivery batch → consumer span. Linked to upstream trace via `session.Traceparent`. |
| `wiring/sqlotel` | manual — replace `sql.NewDb` with `sqlotel.NewDb` in DI | `Begin` / `Query` / `QueryRow` / `Exec` / `Commit` / `Rollback`. `db.statement` set on per-query spans. |

Order matters: put `otel` before `session`. The OTEL middleware writes the new traceparent into Session for outgoing-call propagation — Session must exist by then. If you put `otel` after `session`, OTEL still sees Session (since it's already in ctx) and updates it correctly, but a request without an upstream `traceparent` header gets a fresh trace ID, which is fine.

## Manual spans in business logic

```go
import "go.opentelemetry.io/otel"

func (s *service) DoThing(ctx context.Context) error {
    ctx, span := otel.Tracer("myservice/domain").Start(ctx, "DoThing")
    defer span.End()
    // ... business logic ...
}
```

`otel.Tracer(...)` returns the global TracerProvider, which `tools/otel` installs at startup. With no OTLP endpoint configured, `Start` is a near-zero-cost no-op.

## Tracing and metrics for SQL access

Replace `sql.NewDb` with `sqlotel.NewDb` in your DI config:

```go
import (
    "gitlab.com/gorib/storage/sql"
    "gitlab.com/gorib/waffle/wiring/sqlotel"
)

di.Wire[sql.Db](sqlotel.NewDb, di.Defaults(map[int]any{
    0: env.NeedValue[string]("DB_DRIVER"),
    1: env.NeedValue[string]("DB_DSN"),
}))
```

`trace.TracerProvider` and `metric.MeterProvider` are resolved by DI from `tools/otel`. The decorator emits:

- Per-operation span (`sql.begin` / `sql.query` / `sql.queryRow` / `sql.exec` / `sql.commit` / `sql.rollback`) with `db.statement` attribute
- `db.client.operation.duration` histogram + `db.client.operations` counter, labeled by `db.operation` and `status` (raw query is NEVER a label — would blow cardinality)

## CorrelationId vs trace_id

Waffle keeps two distinct identifiers:

- **`session.CorrelationId`** — a UUID v7 attached to the in-flight request/job. Propagates between services via HTTP headers, gRPC metadata, AMQP headers. Use it to grep logs.
- **`trace_id` / `span_id`** — OTEL's 16/8-byte technical identifiers used by Tempo/Jaeger to render distributed traces.

The two link both ways:

- **Logs → trace**: pry's `Ctx(ctx)` option emits `correlation_id`, `trace_id`, and `span_id` as log fields. pry has no OTEL dependency — it parses the W3C `traceparent` string sitting in `session.Traceparent`. In Tempo, paste the trace_id directly.
- **Trace → logs**: every server span started by `tools/http/otel` and `tools/grpc/otel` carries `correlation_id=<uuid>` as an attribute. From the trace UI you grab the correlation_id and grep your logs.

## Trace context propagation

`session.Session` carries `Traceparent` and `Tracestate` (W3C strings). Transport-layer middleware reads them from incoming headers and writes them to outgoing headers on every transport:

| Transport | Inbound (server) | Outbound (client) |
|-----------|------------------|-------------------|
| HTTP | `http/server/middleware/session.go` reads `traceparent`/`tracestate` headers | `http/client/client.go` writes them on `Do()` |
| gRPC | `grpc/server/interceptor/session.go` reads metadata | `grpc/client/interceptor/session.go` writes metadata |
| AMQP | `amqp/consumer.go` reads message headers | `amqp/publisher.go` writes message headers |

After `tools/http/otel` / `tools/grpc/otel` start the server span, they **overwrite** `session.Traceparent` with the current span's traceparent. That way subsequent outgoing calls in the same handler propagate this span as the parent — not the original upstream parent.

### Inner spans and propagation fidelity

If business logic starts a child span via `otel.Tracer("...").Start(ctx, ...)` and then makes a downstream call, by default the downstream service will see the *server span* as parent (because session.Traceparent still holds the server-span). For exact parent linkage to the child span, use the bridge:

```go
import "gitlab.com/gorib/waffle/wiring/tracectx"

ctx, span := tracectx.StartSpan(ctx, "myservice/domain", "DoThing")
defer span.End()
```

`tracectx.StartSpan` starts the span and mirrors the new traceparent into session, so downstream calls inside that span see *this* span as parent. `tracectx.ExtractIntoContext(ctx)` does the reverse — injects `session.Traceparent` into the OTEL ctx as remote SpanContext (used internally by `tools/scribus/otel`).

### Fan-out goroutines

`tracectx.StartSpan` mutates `*session.Session`. If you fan out into goroutines that share the same ctx and each starts its own span, the writes race. Use `tracectx.ForkCtx(ctx)` to give every goroutine its own copy of Session:

```go
g, gctx := errgroup.WithContext(ctx)
for _, item := range items {
    item := item
    g.Go(func() error {
        gctx := tracectx.ForkCtx(gctx)
        gctx, span := tracectx.StartSpan(gctx, "worker", "process")
        defer span.End()
        return process(gctx, item)
    })
}
```

Without `ForkCtx`, traces still work for the parent goroutine but child-goroutine outgoing calls may propagate the wrong parent span.

## Sampling

Default is `AlwaysSample` — convenient for dev. In production set `OTEL_TRACES_SAMPLER=parentbased_traceidratio` and `OTEL_TRACES_SAMPLER_ARG=0.1` for 10% head-based sampling that respects incoming `traceparent` decisions.

## Not yet covered

- **Redis** — `gorib/storage/redis` is currently a stub; once it has a real implementation, a `wiring/redisotel` decorator will follow.
