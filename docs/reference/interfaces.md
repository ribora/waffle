# Interface Reference

Complete listing of every exported interface in waffle.

---

## app package

### App

```go
type App interface {
    RunUntilStop()
    Run()
    Stop() error
}
```

The top-level application object. Resolved from the DI container in `main.go`.

### HasCloser

```go
type HasCloser interface {
    SetCloser(func() error)
    Closer() func() error
}
```

Implemented by `*app`. Used by the DI container to inject a shutdown function after all dependencies are wired.

### Command

```go
type Command interface {
    BaseCommand
    Run(ctx context.Context) error
}
```

The unit of work. Implement this to add a custom CLI command.

### BaseCommand

```go
type BaseCommand interface {
    Name() string
    Description() string
    Usage() error
}
```

Identity and help text for a command. Tool constructors accept `BaseCommand` so name/description are provided by the framework.

---

## tools/http package

### HttpCommand

```go
type HttpCommand interface {
    app.Command
}
```

Marker interface for the HTTP server command. Registered under `app.CommandTag`.

HTTP middleware is a `server.Layer` (`func(next server.Handler) server.Handler`), not a tool-specific interface. The standard chain ships as `*middleware.Standard` and is applied in the router's `Setup` via `mux.Use(standard.All()...)`; see [tools/http](../tools/http.md#middleware).

---

## tools/grpc package

### GrpcCommand

```go
type GrpcCommand interface {
    app.Command
}
```

Marker interface for the gRPC server command.

### ServiceHandler

```go
type ServiceHandler interface {
    Start(grpc.ServiceRegistrar)
}
```

Implemented by each gRPC domain handler. `Start` registers the proto-generated server with the gRPC registrar.

### Interceptor

```go
// gitlab.com/gorib/grpc/server
type Interceptor interface {
    Name() string
    Order() int
    Run() grpc.UnaryServerInterceptor
}
```

A unary gRPC interceptor. Tag with `grpc.MiddlewareTag`. Chained by `Order()` ascending (smaller = outer). Reserved framework orders: 100 tracing, 200 status, 300 session, 400 log, 500 recover.

### StreamInterceptor

```go
// gitlab.com/gorib/grpc/server
type StreamInterceptor interface {
    Name() string
    Order() int
    Run() grpc.StreamServerInterceptor
}
```

A stream gRPC interceptor. Tag with `grpc.StreamMiddlewareTag`.

### ErrorMapper

```go
// gitlab.com/gorib/grpc/server
type ErrorMapper interface {
    Errors() map[error]codes.Code
}
```

A standalone error→gRPC code mapping. Tag with `grpc.ErrorMapperTag`. The framework aggregates all registered mappers; the built-in `status` and `log` interceptors read the merged map at request time. The same map covers both unary and stream chains. See [tools/grpc.md](../tools/grpc.md#error-mapping) for the full pattern.

---

## tools/migrate package

### MigrateCommand

```go
type MigrateCommand interface {
    app.Command
}
```

Marker interface for the migration runner command.

---

## tools/scribus package

### ScribusCommand

```go
type ScribusCommand interface {
    app.Command
}
```

Marker interface for the RabbitMQ consumer command.

### Handler

```go
type Handler interface {
    Keys() []string
    Run(src amqp.Source) error
}
```

A single-mode message handler. `Keys()` declares which routing keys this handler subscribes to. Tag with `scribus.CommandTag`. Parse the body with `scan.Into[T](src)`.

### BatchHandler

```go
type BatchHandler interface {
    Keys() []string
    Run(msgs []amqp.Source) error
}
```

A batch-mode message handler. Receives a slice of `amqp.Source` flushed together. Provide exactly one without a tag.

### Consumer

```go
type Consumer interface {
    SetHandler(handler func(msgs []amqp.Source) error) error
    SetDsn(dsn string) error
    SetExchange(exchange string) error
    SetQueue(queue string) error
    SetRoutingKey(routingKey string) error
    SetPageSize(pageSize uint16) error
    SetFlushInterval(flushInterval time.Duration) error
    SetReconnectInterval(interval time.Duration) error
    Consume(ctx context.Context) error
}
```

The AMQP consumer from `gitlab.com/gorib/amqp`. Configured and started by `scribusCommand.Run`. You do not implement this — it is provided by the registrations in `tools/scribus/init.go` (see [Scribus Tool](../tools/scribus.md) for setup).
