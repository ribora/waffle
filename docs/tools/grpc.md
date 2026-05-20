# gRPC Tool

The gRPC tool wraps `gitlab.com/gorib/grpc/server` into a waffle `Command`. Pull in its DI registrations via `di.Include` to register the `grpc` command.

## Setup

Add a single `di.Include` to `InitDi()` in `config/di/di.go`:

```go
import "gitlab.com/gorib/di"

func InitDi() {
    di.Include("gitlab.com/gorib/waffle/tools/grpc")
    // ... other registrations ...
}
```

This pulls in the registrations from [`tools/grpc/init.go`](../../tools/grpc/init.go), which is equivalent to writing them yourself:

```go
import (
    "gitlab.com/gorib/di"
    "gitlab.com/gorib/waffle/app"
    wgrpc "gitlab.com/gorib/waffle/tools/grpc"
)

func InitDi() {
    di.Wire[app.BaseCommand](
        func() app.BaseCommand { return app.NewCommand("grpc", "Start grpc server") },
        di.For[wgrpc.GrpcCommand](),
    )
    di.Wire[wgrpc.GrpcCommand](wgrpc.NewGrpcCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
        "commands":           di.Tags[wgrpc.ServiceHandler](wgrpc.ServiceTag),
        "interceptors":       di.Tags[wgrpc.Middleware](wgrpc.MiddlewareTag),
        "streamInterceptors": di.Tags[wgrpc.StreamMiddleware](wgrpc.StreamMiddlewareTag),
    }))
}
```

Use the manual form when you need to override a registration (e.g. rename the command); otherwise prefer `di.Include`.

Run `make generate`, then start the server:

```sh
./app grpc
./app grpc --listen :50051
```

## Listen address

`NewGrpcCommand` requires a `net.Addr` argument. You must wire it explicitly in `InitDi()`:

```go
import "net"

// Inside InitDi():
di.Wire[net.Addr](func() (net.Addr, error) {
    return net.ResolveTCPAddr("tcp", env.Value[string]("...", ":50051"))
}, di.For[wgrpc.GrpcCommand]())
```

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
type Middleware interface {
    Name() string
    Order() int
    Run() grpc.UnaryServerInterceptor
}
```

Reserved conventional orders: 100 tracing, 200 status, 300 session, 400 log, 500 recover. Pick `Order()` ≥ 1000 for user interceptors.

```go
type LoggingMiddleware struct {
    logger pry.Logger
}

func (LoggingMiddleware) Name() string { return "audit" }
func (LoggingMiddleware) Order() int   { return 1000 }
func (m *LoggingMiddleware) Run() grpc.UnaryServerInterceptor {
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
di.Define(NewLoggingMiddleware, di.Tag(grpcCommand.MiddlewareTag))
```

### Stream interceptor

```go
type StreamMiddleware interface {
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
type ErrorMapper interface {
    Errors() map[error]codes.Code
}
```

`ErrorMapper` is collected from **interceptor** values, not from service handlers. The recommended pattern is a dedicated error handler registered under `grpcCommand.MiddlewareTag`.

A middleware can implement `ErrorMapper` purely for error code mapping — `Run()` is required to satisfy the `Middleware` interface but can be a pass-through no-op:

```go
func NewErrorHandler() *errorHandler { return &errorHandler{} }

type errorHandler struct{}

func (m *errorHandler) Errors() map[error]codes.Code {
    return map[error]codes.Code{
        model.ErrUserNotFound: codes.NotFound,
        model.ErrUserConflict: codes.AlreadyExists,
    }
}

func (errorHandler) Name() string  { return "error-mapper" }
func (errorHandler) Order() int    { return 2000 }
// Run is a pass-through; the actual mapping happens via Errors() above.
func (m *errorHandler) Run() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        return handler(ctx, req)
    }
}
```

Register it as an interceptor:

```go
di.Define(NewErrorHandler, di.Tag(grpcCommand.MiddlewareTag))
```

The gRPC tool collects all `Errors()` maps from registered interceptors that implement `ErrorMapper`, merges them into the shared `*server.ErrorMap`, and the built-in `status` interceptor reads it at request time.

### StreamErrorMapper

```go
type StreamErrorMapper interface {
    StreamErrors() map[error]codes.Code
}
```

Same pattern for stream RPCs — implement on an interceptor registered under `grpcCommand.StreamMiddlewareTag`.

## DI tags

| Tag | Collects |
|-----|---------|
| `grpc.ServiceTag` | All `ServiceHandler` implementations |
| `grpc.MiddlewareTag` | All `Middleware` (unary) implementations |
| `grpc.StreamMiddlewareTag` | All `StreamMiddleware` implementations |

## Graceful shutdown

On `SIGINT` / `SIGTERM`, the gRPC server calls `GracefulStop()`, which waits for active RPCs to complete before closing.
