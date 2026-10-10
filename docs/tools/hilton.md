# Hilton (tracing & metrics)

[Docs](../README.md) · Tools

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
| `HOSTNAME` | `service.instance.id` resource attribute; set by the container runtime in practice |

All four are `env.Value`, so none of them stops a boot — a service with no
observability configured runs and reports nothing.

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
| Redis | `Client` | `db.client.operation.duration` |
| SQL | `Client` | `db.client.operation.duration` |
| Elasticsearch | `Client` | `db.client.operation.duration` |
| CLI command | `Internal` | `cli.command.duration` |

Every duration is in seconds and carries explicit bucket boundaries — the SDK
default is millisecond-scaled and would put every request in one bucket.

W3C `traceparent` is propagated across all boundaries via the active
`hilton.Propagator`. A batch AMQP consumer takes its parent from one message and
links to the rest, so no message's trace loses the connection.

## `error.type`

Every duration above carries `error.type` when the operation did not succeed,
and omits the key entirely when it did — one present on every success is one
nobody can group by.

The value is chosen the same way everywhere, in order of preference:

1. the transport's own classification, when it has one — the gRPC code, the HTTP
   status, Elasticsearch's exception type, a Redis reply code;
2. `failure.Type()` when the error is a `*failure.Error` — a slug derived from
   the sentinel's format string, so the cardinality is bounded by the number of
   declared sentinels;
3. `_OTHER`, which is semconv's own fallback.

Never the Go type of the error: it classifies nothing, and it changes whenever
someone changes how the error is wrapped.

| Layer | Values |
|-------|--------|
| HTTP server, client | the status (`404`, `502`), else the failure type, else `_OTHER` |
| gRPC server, client | the code (`NotFound`, `Unavailable`) |
| AMQP consumer | the failure type, else `_OTHER`; a caught panic is `panic` |
| Redis | `no_rows`, `wrong_type`, `cluster`, `auth`, `timeout`, `connect`, … else `_OTHER` |
| SQL | `conflict`, `deadlock`, `no_rows`, `bad_request`, `no_column`, `internal`, `connect`, `tx`, else `_OTHER` |
| Elasticsearch | the cluster's exception type (`index_not_found_exception`), else `wrong_response`, `unknown_response` or `_OTHER` |
| CLI command | `usage`, else the failure type, else `_OTHER` |

A lookup that found nothing is counted but not failed: SQL's `ErrNoRows` and
Redis's `redis.Nil` both report `no_rows` on `db.client.operation.duration` and
neither marks its span as an error, so a cold cache does not read as an outage.
`db.system.name` tells the two apart.

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

## Next

| | |
|---|---|
| How the provider reaches a CLI command's context | [core/context](../core/context.md) |
| The `cli.command.duration` histogram and the span around every command | [core/application](../core/application.md#run) |
| Why `hilton` is one of the four packages you cannot really remove | [What is optional](../opting-out.md#the-core-you-cannot-really-remove) |
| Every variable the otel module reads | [reference/environment](../reference/environment.md#configservicehiltonotel) |
| Running without a collector | [reference/running-locally](../reference/running-locally.md#when-it-does-not-start) |
