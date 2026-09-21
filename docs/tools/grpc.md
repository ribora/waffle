# gRPC Tool

[Docs](../README.md) · Tools

The gRPC tool wraps `gitlab.com/gorib/grpc/server` into a waffle `Command`. Blank-import its config package to register the `grpc` command.

## Setup

Add a single blank import to `config/di/modules.go`:

```go
// config/di/modules.go
package di

import (
    _ "gitlab.com/gorib/waffle/config/tools/grpc"
    // ... other modules ...
)
```

This pulls in the registrations from [`config/tools/grpc/init.go`](../../config/tools/grpc/init.go), which is equivalent to writing them yourself:

```go
import (
    "net"
    "strconv"

    "gitlab.com/gorib/di"
    "gitlab.com/gorib/env"
    "gitlab.com/gorib/grpc/server"
    "gitlab.com/gorib/grpc/server/interceptor"
    "gitlab.com/gorib/waffle/app"
    wgrpc "gitlab.com/gorib/waffle/tools/grpc"
)

func InitDi() {
    di.Wire[server.ErrorMapper](wgrpc.NewErrorMap, di.Args(map[string]any{
        "mappers": di.Tags(wgrpc.ErrorMapperTag),
    }))

    di.Define(interceptor.NewTracingMW, di.Tag(wgrpc.MiddlewareTag))
    di.Define(interceptor.NewStatusMW, di.Tag(wgrpc.MiddlewareTag))
    di.Define(interceptor.NewSessionMW, di.Tag(wgrpc.MiddlewareTag))
    di.Define(interceptor.NewLogMW, di.Tag(wgrpc.MiddlewareTag))
    di.Define(interceptor.NewRecoverMW, di.Tag(wgrpc.MiddlewareTag))

    // ... and the five NewStream…MW siblings under StreamMiddlewareTag ...

    di.Wire[net.Addr](net.ResolveTCPAddr, di.For[wgrpc.GrpcCommand](), di.Args(map[string]any{
        "network": "tcp",
        "address": net.JoinHostPort(
            env.NeedValue[string]("DAEMON_HOST"),
            strconv.FormatUint(uint64(env.NeedValue[uint16]("DAEMON_PORT")), 10),
        ),
    }))

    di.Wire[app.BaseCommand](
        func() app.BaseCommand { return app.NewCommand("grpc", "Start grpc server") },
        di.For[wgrpc.GrpcCommand](),
    )
    di.Wire[wgrpc.GrpcCommand](wgrpc.NewGrpcCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
        "commands":           di.Tags(wgrpc.ServiceTag),
        "interceptors":       di.Tags(wgrpc.MiddlewareTag),
        "streamInterceptors": di.Tags(wgrpc.StreamMiddlewareTag),
    }))
}
```

Use the manual form when you need to override a registration (e.g. rename the command); otherwise prefer the blank import.

Run `make generate`, then start the server:

```sh
./app grpc
./app grpc --listen :50051
```

## Listen address

`NewGrpcCommand` takes a `net.Addr`, and the module wires it from `DAEMON_HOST` and `DAEMON_PORT` — both `env.NeedValue`, so a missing one stops the process at startup rather than at the first call. You wire it yourself only to take the address from somewhere else; `di.For[wgrpc.GrpcCommand]()` is what keeps that registration off every other `net.Addr` in the container.

The HTTP module reads the same pair, so the two servers are two commands of one binary rather than two listeners in one process — see [reference/running-locally](../reference/running-locally.md#two-servers-at-once).

Override the address at runtime with the `--listen` / `-l` flag (takes precedence over the wired value):

```sh
./app grpc --listen 0.0.0.0:50051
# or short form
./app grpc -l :50051
```

## Service handlers

### ServiceHandler interface

```go
type ServiceHandler interface {
    Start(grpc.ServiceRegistrar)
}
```

Each domain that exposes gRPC implements `ServiceHandler`. `waffle make:domain:grpc` generates this for you. The `Start` method receives the gRPC server registrar and calls the protoc-generated `RegisterXxxServer` function:

```go
func (g *UserGrpc) Start(registrar grpc.ServiceRegistrar) {
    proto.RegisterUserServiceServer(registrar, g)
}
```

### Registering a handler

Use `di.Define` tagged with `grpcCommand.ServiceTag`. Handlers are concrete types — do not use `di.Wire[ServiceHandler]`:

```go
di.Define(usergrpc.NewUserGrpc, di.Tag(grpcCommand.ServiceTag))
```

All tagged handlers are passed to `NewGrpcCommand` and registered with the server on startup.

## Interceptors

Built-in interceptors (tracing, status, session, log, recover) are wired automatically by `tools/grpc`. User-defined interceptors are chained alongside them, ordered by `Order()` ascending (smaller = outer).

### Unary interceptor

```go
// gitlab.com/gorib/grpc/server
type Interceptor interface {
    Name() string
    Order() int
    Run() grpc.UnaryServerInterceptor
}
```

The five that ship occupy fixed positions. Pick `Order()` ≥ 1000 for your own.

| Order | Interceptor | Why there |
|---|---|---|
| 100 | `session` | builds the `session.Session` everything below it logs and traces under |
| 200 | `tracing` | starts the `SERVER` span and records `rpc.server.duration` |
| 300 | `log` | traces request and response, and picks trace vs error level by status code |
| 400 | `status` | maps a domain error to a gRPC code and attaches the `failure` detail |
| 500 | `recover` | turns a panic into `failure.Panic` |

**The order is load-bearing in both directions, and getting it wrong is silent.**
`status` sits **inside** tracing and log so that both observe the mapped code
rather than the raw error — outside them, every failure is recorded as
`Unknown`, which this repository has already shipped once. `recover` sits inside
`status` so a panic becomes an error `status` can then map, instead of killing
the process.

That is also why `UnaryChain` is exported: a component test chains the
container's own interceptor list with the same function the daemon uses, rather
than sorting a copy. See [conventions/component-harness](../conventions/component-harness.md#the-in-process-server).

The full table, with what each one puts on the context and which methods skip
tracing, is [Interceptor order](https://gitlab.com/gorib/grpc#interceptor-order)
in the `grpc` README. The stream set has the same five orders.

```go
type LoggingInterceptor struct {
    logger pry.Logger
}

func (LoggingInterceptor) Name() string { return "audit" }
func (LoggingInterceptor) Order() int   { return 1000 }
func (m *LoggingInterceptor) Run() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        m.logger.Info(info.FullMethod, pry.Ctx(ctx))
        resp, err := handler(ctx, req)
        if err != nil {
            m.logger.Error(err, pry.Ctx(ctx))
        }
        return resp, err
    }
}
```

Tag with `grpcCommand.MiddlewareTag` using `di.Define`:

```go
di.Define(NewLoggingInterceptor, di.Tag(grpcCommand.MiddlewareTag))
```

### Stream interceptor

```go
// gitlab.com/gorib/grpc/server
type StreamInterceptor interface {
    Name() string
    Order() int
    Run() grpc.StreamServerInterceptor
}
```

Tag with `grpcCommand.StreamMiddlewareTag` using `di.Define`.

## Error mapping

gRPC returns status codes. Map domain errors to codes so callers receive meaningful responses instead of `codes.Unknown`.

### ErrorMapper interface

```go
// gitlab.com/gorib/grpc/server
type ErrorMapper interface {
    Errors() map[error]codes.Code
}
```

`ErrorMapper` is a standalone concept — not coupled to interceptors. Register implementations under `grpcCommand.ErrorMapperTag`; the framework aggregates them into a single `ErrorMapper` that the built-in `status` and `log` interceptors read at request time.

```go
type domainErrors struct{}

func NewDomainErrors() *domainErrors { return &domainErrors{} }

func (domainErrors) Errors() map[error]codes.Code {
    return map[error]codes.Code{
        model.ErrUserNotFound: codes.NotFound,
        model.ErrUserConflict: codes.AlreadyExists,
    }
}
```

Register:

```go
di.Define(NewDomainErrors, di.Tag(grpcCommand.ErrorMapperTag))
```

The same `ErrorMapper` instance feeds both unary and stream `status` interceptors — there's no separate `StreamErrorMapper`.

## DI tags

| Tag | Collects |
|-----|---------|
| `grpc.ServiceTag` | All `ServiceHandler` implementations |
| `grpc.MiddlewareTag` | All `Interceptor` (unary) implementations |
| `grpc.StreamMiddlewareTag` | All `StreamInterceptor` implementations |
| `grpc.ErrorMapperTag` | All `ErrorMapper` implementations |

## Graceful shutdown

On `SIGINT` / `SIGTERM`, the gRPC server calls `GracefulStop()`, which waits for active RPCs to complete before closing.

## Next

| | |
|---|---|
| Writing the handler this server registers | [tools/grpc-handler](grpc-handler.md) |
| Scaffolding the proto, the handler and its DI | [cli/make:domain:grpc](../cli/make-domain-grpc.md) |
| Converting between proto and domain types | [conventions/dto-conversion](../conventions/dto-conversion.md) |
| `ServiceTag` and the three other collections | [di/tags](../di/tags.md#grpcservicetag) |
| `ServiceHandler`, `Interceptor`, `ErrorMapper` in full | [reference/interfaces](../reference/interfaces.md#toolsgrpc-package) |
| What has to be in `.env`, and running http and grpc at once | [reference/running-locally](../reference/running-locally.md#two-servers-at-once) |
| Calling the real server from a component test | [conventions/component-harness](../conventions/component-harness.md#the-in-process-server) |
