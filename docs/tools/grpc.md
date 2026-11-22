# gRPC Tool

[Docs](../README.md) · Tools

The gRPC tool wraps `github.com/ribora/grpc/server` into a waffle `Command`. Blank-import its config package to register the `grpc` command.

## Setup

Add a single blank import to `config/di/modules.go`:

```go
// config/di/modules.go
package di

import (
    _ "github.com/ribora/waffle/config/tools/grpc"
    // ... other modules ...
)
```

This pulls in the registrations from [`config/tools/grpc/init.go`](../../config/tools/grpc/init.go), which is equivalent to writing them yourself:

```go
import (
    "net"
    "strconv"

    "github.com/ribora/diogen/di"
    "github.com/ribora/env"
    "github.com/ribora/grpc/server"
    "github.com/ribora/grpc/server/interceptor"
    "github.com/ribora/waffle/app"
    wgrpc "github.com/ribora/waffle/tools/grpc"
)

func InitDi() {
    di.Wire[server.ErrorMapper](wgrpc.NewErrorMap, di.Args(map[string]any{
        "mappers": di.Tags(wgrpc.ErrorMapperTag),
    }))

    session := di.Define(interceptor.NewSessionMW)
    tracing := di.Define(interceptor.NewTracingMW)
    logging := di.Define(interceptor.NewLogMW)
    status := di.Define(interceptor.NewStatusMW)
    recovering := di.Define(interceptor.NewRecoverMW)

    di.Wire[server.Interceptors](wgrpc.NewInterceptors, di.Fallback(), di.Args(map[string]any{
        "interceptors": di.List(session, tracing, logging, status, recovering),
    }))

    // ... and the five NewStream…MW siblings, as server.StreamInterceptors ...

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
        "commands":       di.Tags(wgrpc.ServiceTag),
        "maxRecvMsgSize": env.Value("GRPC_MAX_RECV_MSG_SIZE", 4<<20),
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

## Message size

`GRPC_MAX_RECV_MSG_SIZE` caps the size of a request, in bytes. Unset, it is
gRPC's own 4 MiB, and a larger request is refused with `ResourceExhausted`
before any handler runs.

## Service handlers

### ServiceHandler interface

```go
type ServiceHandler interface {
    Start(grpc.ServiceRegistrar)
}
```

Each domain that exposes gRPC implements `ServiceHandler`. `waffle make:domain:grpc` generates this for you. The `Start` method receives the gRPC server registrar and calls the protoc-generated `RegisterXxxServer` function:

```go
func (s *userService) Start(registrar grpc.ServiceRegistrar) {
    proto.RegisterUserServiceServer(registrar, s)
}
```

### Registering a handler

Use `di.Define` tagged with `grpcCommand.ServiceTag`. Handlers are concrete types — do not use `di.Wire[ServiceHandler]`:

```go
di.Define(userGrpc.NewUserService, di.Tag(grpcCommand.ServiceTag))
```

All tagged handlers are passed to `NewGrpcCommand` and registered with the server on startup.

## Interceptors

Built-in interceptors (session, tracing, log, status, recover) are wired automatically by `config/tools/grpc`, as a `di.List` under `server.Interceptors`. The list is the order.

### Unary interceptor

```go
// github.com/ribora/grpc/server
type Interceptor interface {
    Run() grpc.UnaryServerInterceptor
}
```

The five that ship, outermost first:

| Interceptor | Why there |
|---|---|
| `session` | builds the `session.Session` everything below it logs and traces under |
| `tracing` | starts the `SERVER` span and records `rpc.server.duration` |
| `log` | traces request and response, and picks trace vs error level by status code |
| `status` | maps a domain error to a gRPC code and attaches the `failure` detail |
| `recover` | turns a panic into `failure.Panic` |

**The order is load-bearing in both directions, and getting it wrong is silent.**
`status` sits **inside** tracing and log so that both observe the mapped code
rather than the raw error — outside them, every failure is recorded as
`Unknown`, which this repository has already shipped once. `recover` sits inside
`status` so a panic becomes an error `status` can then map, instead of killing
the process.

That is also why `UnaryChain` is exported: a component test chains the
container's own interceptor list with the same function the daemon uses. See
[Testing](#testing).

The full table, with what each one puts on the context and which methods skip
tracing, is [Interceptor order](https://github.com/ribora/grpc#interceptor-order)
in the `grpc` README. The stream set belongs in the same order.

```go
type LoggingInterceptor struct {
    logger pry.Logger
}

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

Write your own list, with yours in the position you want. Naming the shipped
constructors again is how you keep them — the default list then feeds nobody and
drops out of the container whole:

```go
audit := di.Define(NewLoggingInterceptor)
session := di.Define(interceptor.NewSessionMW)
// ... the rest you want to keep

di.Wire[server.Interceptors](wgrpc.NewInterceptors, di.Args(map[string]any{
    "interceptors": di.List(session, audit /* ... */),
}))
```

### Stream interceptor

```go
// github.com/ribora/grpc/server
type StreamInterceptor interface {
    Run() grpc.StreamServerInterceptor
}
```

Same again, under `server.StreamInterceptors`.

## Error mapping

gRPC returns status codes. Map domain errors to codes so callers receive meaningful responses instead of `codes.Unknown`.

### ErrorMapper interface

```go
// github.com/ribora/grpc/server
type ErrorMapper interface {
    Errors() map[error]codes.Code
}
```

`ErrorMapper` is a standalone concept — not coupled to interceptors. Register implementations under `grpcCommand.ErrorMapperTag`; the framework aggregates them into a single `ErrorMapper` that the built-in `status` and `log` interceptors read once, when they are built.

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
| `grpc.ErrorMapperTag` | All `ErrorMapper` implementations |

Interceptors are not collected by a tag: their order is a contract, so they are
listed under `server.Interceptors` instead.

## Graceful shutdown

On `SIGINT` / `SIGTERM`, the gRPC server calls `GracefulStop()`, which waits for active RPCs to complete before closing.

A call that does not end on its own — a subscription, a watch — would hold that wait for good. A handler serving one implements `Drainer` beside `ServiceHandler`:

```go
type Drainer interface {
    Drain()
}
```

The command calls `Drain` on every handler that has it while `GracefulStop` waits, so `Drain` ends such calls the way they end normally, not with an error. The server takes no new call by then, but one can slip in at the moment it stops listening: a call of that kind that comes after `Drain` ends at once. The command drains its handlers when it closes without having served, too, so a component test that closes the container reaches `Drain`.

## Testing

`waffleGrpc.Serve` (`github.com/ribora/waffle/tools/grpc`) runs the handlers on an in-process server over bufconn. It chains the container's own interceptor lists with `UnaryChain` and `StreamChain`, the functions the daemon uses, so a test checks the chain that ships:

```go
server, err := waffleGrpc.Serve(
    container.GrpcCommandServiceHandlerAsServiceTag(),
    container.ServerInterceptorsInterceptors(),
    container.ServerStreamInterceptorsInterceptors(),
)
if err != nil {
    t.Fatal(err)
}
defer server.Close()

user, err := proto.NewUserServiceClient(server.Conn()).Get(t.Context(), &proto.GetUserRequest{Id: 7})
```

The getters follow the import aliases the scaffold writes.

A neighbouring service is cut at its typed client, not at `ClientConnInterface`. `grpcClient.Dead(name)` (`github.com/ribora/grpc/client`) fills the connection under a client left real: a call that reaches it panics, naming it. Building `container` and choosing what to replace is [Testing with the container](../di/overview.md#testing-with-the-container).

## Next

| | |
|---|---|
| Writing the handler this server registers | [tools/grpc-handler](grpc-handler.md) |
| Scaffolding the proto, the handler and its DI | [cli/make:domain:grpc](../cli/make-domain-grpc.md) |
| Converting between proto and domain types | [conventions/dto-conversion](../conventions/dto-conversion.md) |
| `ServiceTag` and every other collection | [di/tags](../di/tags.md#grpcservicetag) |
| `ServiceHandler`, `Interceptor`, `ErrorMapper` in full | [reference/interfaces](../reference/interfaces.md#toolsgrpc-package) |
| What has to be in `.env`, and running http and grpc at once | [reference/running-locally](../reference/running-locally.md#two-servers-at-once) |
