# Hilton (tracing & metrics)

`gitlab.com/gorib/hilton` is a thin tracing/metrics abstraction. Base packages depend on hilton interfaces, not on any specific backend; the OTel SDK is the default implementation, hidden behind the `hilton.Provider` interface.

## Setup

One include enables the whole stack:

```go
func InitDi() {
    di.Include("gitlab.com/gorib/waffle/tools/hilton")
    di.Include("gitlab.com/gorib/waffle/tools/http")
    di.Include("gitlab.com/gorib/waffle/tools/grpc")
}
```

`tools/hilton` wires a single `hilton.Provider` from env-vars. Other tools — `tools/http`, `tools/grpc`, and any I/O client (HTTP client, AMQP, SQL) — pick it up automatically.

## Configuration

| Variable | Purpose |
|----------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP/gRPC collector endpoint (e.g. `otel-collector:4317`). Empty → spans/metrics are created and propagated but not exported. |
| `SERVICE` | `service.name` resource attribute |
| `ENVIRONMENT` | `deployment.environment` resource attribute |

## Provider lifecycle

`hilton.Provider` is wired as a DI singleton and injected into:

- HTTP server tracing middleware (sets it on request ctx)
- gRPC server tracing interceptor (sets it on request ctx)
- `app.NewContextWithCommand` (sets it on the root ctx of every CLI command)

Once on ctx, any I/O code path reads it via `hilton.ProviderFromContext(ctx)`. Absent provider → `hilton.Noop()`.

## What gets instrumented

| Layer | Span kind | Metrics |
|-------|-----------|---------|
| HTTP server | `Server` | `http.server.request.duration` |
| HTTP client | `Client` | (span only) |
| gRPC server | `Server` | `rpc.server.duration`, `rpc.server.requests` |
| gRPC client | `Client` | `rpc.client.duration`, `rpc.client.requests` |
| AMQP publisher | `Producer` | (span only) |
| AMQP consumer | `Consumer` | (span only) |
| SQL (`sql.NewTracedDb`) | `Client` | `db.client.operation.duration`, `db.client.operations` |

W3C `traceparent` is propagated across all boundaries via the active `hilton.Propagator`.

## Custom instrumentation

Inside a request-handling code path:

```go
func (s *service) DoWork(ctx context.Context) error {
    ctx, span := hilton.ProviderFromContext(ctx).Tracer("my-service").Start(ctx, "DoWork",
        hilton.WithSpanKind(hilton.SpanKindInternal),
    )
    defer span.End()
    // ...
}
```
