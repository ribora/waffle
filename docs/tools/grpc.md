# gRPC Tool

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
    "gitlab.com/gorib/di"
    "gitlab.com/gorib/grpc/server"
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
        "interceptors":       di.Tags[server.Interceptor](wgrpc.MiddlewareTag),
        "streamInterceptors": di.Tags[server.StreamInterceptor](wgrpc.StreamMiddlewareTag),
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
// gitlab.com/gorib/grpc/server
type Interceptor interface {
    Name() string
    Order() int
    Run() grpc.UnaryServerInterceptor
}
```

Reserved conventional orders: 100 tracing, 200 status, 300 session, 400 log, 500 recover. Pick `Order()` ≥ 1000 for user interceptors.

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
