# gRPC Tool

The gRPC tool wraps `gitlab.com/gorib/grpc/server` into a waffle `Command`. Call `InitGrpc()` in your DI configuration to register the `grpc` command.

## Setup

Add the following registrations inside `InitDi()` in `config/di/di.go`:

```go
import (
    "gitlab.com/gorib/di"
    "gitlab.com/gorib/waffle/app"
    wgrpc "gitlab.com/gorib/waffle/tools/grpc"
)

func InitDi() {
    // ... other registrations ...

    di.Wire[app.BaseCommand](
        func() app.BaseCommand { return app.NewCommand("grpc", "Start grpc server") },
        di.For[wgrpc.GrpcCommand](),
    )
    di.Wire[wgrpc.GrpcCommand](wgrpc.NewGrpcCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
        "commands":          di.Tags[wgrpc.ServiceHandler](wgrpc.ServiceTag),
        "middlewares":       di.Tags[wgrpc.Middleware](wgrpc.MiddlewareTag),
        "streamMiddlewares": di.Tags[wgrpc.StreamMiddleware](wgrpc.StreamMiddlewareTag),
    }))
}
```

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

## Middleware

### Unary middleware

```go
type Middleware interface {
    Run() grpc.UnaryServerInterceptor
}
```

Implement for request logging, authentication, panic recovery, etc:

```go
type LoggingMiddleware struct {
    logger pry.Logger
}

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

### Stream middleware

```go
type StreamMiddleware interface {
    Run() grpc.StreamServerInterceptor
}
```

Tag with `grpcCommand.StreamMiddlewareTag` using `di.Define`:

```go
di.Define(NewStreamLoggingMiddleware, di.Tag(grpcCommand.StreamMiddlewareTag))
```

## Error mapping

gRPC returns status codes. Map domain errors to codes so callers receive meaningful responses instead of `codes.Unknown`.

### ErrorMapper interface

```go
type ErrorMapper interface {
    Errors() map[error]codes.Code
}
```

`ErrorMapper` is collected from **middleware** values, not from service handlers. The recommended pattern is a dedicated error handler registered under `grpcCommand.MiddlewareTag`.

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

// Run satisfies the Middleware interface but does no interception itself.
// The actual error mapping happens via Errors() above.
func (m *errorHandler) Run() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        return handler(ctx, req)
    }
}
```

Register it as middleware:

```go
di.Define(NewErrorHandler, di.Tag(grpcCommand.MiddlewareTag))
```

The gRPC tool collects all `Errors()` maps from registered middlewares that implement `ErrorMapper`, merges them, and uses the combined map in the error-handling interceptor.

### StreamErrorMapper

```go
type StreamErrorMapper interface {
    StreamErrors() map[error]codes.Code
}
```

Same pattern for stream RPCs — implement on a middleware registered under `grpcCommand.StreamMiddlewareTag`.

## DI tags

| Tag | Collects |
|-----|---------|
| `grpc.ServiceTag` | All `ServiceHandler` implementations |
| `grpc.MiddlewareTag` | All `Middleware` (unary) implementations |
| `grpc.StreamMiddlewareTag` | All `StreamMiddleware` implementations |

## Graceful shutdown

On `SIGINT` / `SIGTERM`, the gRPC server calls `GracefulStop()`, which waits for active RPCs to complete before closing.
