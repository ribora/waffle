# Hilton (tracing & metrics)

`gitlab.com/gorib/hilton` is a thin tracing/metrics abstraction. Base packages depend on hilton interfaces, not on any specific backend; the OTel SDK is the default implementation, hidden behind the `hilton.Provider` interface.

## Setup

Add a blank import for `config/service/hilton/otel` to your `modules.go`:

```go
// config/di/modules.go
package di

import (
    _ "gitlab.com/gorib/waffle/config/app"                 // wires app.App
    _ "gitlab.com/gorib/waffle/config/service/hilton/otel" // OTel observability
    _ "gitlab.com/gorib/waffle/config/service/pry"         // logging
    _ "gitlab.com/gorib/waffle/config/tools/http"
    _ "gitlab.com/gorib/waffle/config/tools/grpc"
)
```

Once `service/hilton/otel` is in `modules.go`, `hilton.Provider` is wired automatically and injected into every HTTP/gRPC handler context and CLI command root. With empty `OTEL_EXPORTER_OTLP_ENDPOINT` the provider is a noop — spans/metrics are created and propagated but not exported. No conditional plumbing in service code.

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
| HTTP server | `Server` | `http.server.request.duration`, `http.server.active_requests` |
| HTTP client | `Client` | `http.client.request.duration` |
| gRPC server | `Server` | `rpc.server.duration`, `rpc.server.active_calls` |
| gRPC client | `Client` | `rpc.client.duration` |
| AMQP publisher | `Producer` | (span only) |
| AMQP consumer | `Consumer` | `messaging.process.duration`, `messaging.process.active`, `messaging.client.consumed.messages` (counts individual messages in batches) |
| Redis | `Client` | `db.client.operation.duration`, `db.client.connection.count`, `db.client.connection.pending_requests` |
| SQL | `Client` | `db.client.operation.duration`, `db.client.connection.count` |
| Elasticsearch | `Client` | `db.client.operation.duration` |
| CLI command | `Internal` | `cli.command.duration` |

Every duration is in seconds and carries explicit bucket boundaries — the SDK
default is millisecond-scaled and would put every request in one bucket.

Pool gauges are wired separately from the pool itself (`sql.NewPoolTracing`,
`redis.NewPoolTracing`): they are read at collection time, so they need a
provider for the pool's lifetime rather than the per-operation one from the
context. Both are `io.Closer`, so the container unregisters them on shutdown.

W3C `traceparent` is propagated across all boundaries via the active
`hilton.Propagator`. A batch AMQP consumer takes its parent from one message and
links to the rest, so no message's trace loses the connection.

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
