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

### Middleware

```go
type Middleware interface {
    Name() string
    Run() fiber.Handler
}
```

An HTTP middleware. `Name()` is used as the lookup key in `MiddlewareList`. Implement and tag with `http.MiddlewareTag`.

### MiddlewareList

```go
type MiddlewareList interface {
    Get(name string) fiber.Handler
}
```

A registry of named HTTP middlewares. Injected into the router constructor. `Get` panics if the name is not found.

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

### Middleware

```go
type Middleware interface {
    Run() grpc.UnaryServerInterceptor
}
```

A unary gRPC interceptor. Tag with `grpc.MiddlewareTag`.

### StreamMiddleware

```go
type StreamMiddleware interface {
    Run() grpc.StreamServerInterceptor
}
```

A stream gRPC interceptor. Tag with `grpc.StreamMiddlewareTag`.

### ErrorMapper

```go
type ErrorMapper interface {
    Errors() map[error]codes.Code
}
```

Optionally implemented by `Middleware` to map domain errors to gRPC status codes. The gRPC tool collects all `Errors()` maps from registered middlewares and merges them into the error-handling interceptor. See [tools/grpc.md](../tools/grpc.md#error-mapping) for the full pattern.

### StreamErrorMapper

```go
type StreamErrorMapper interface {
    StreamErrors() map[error]codes.Code
}
```

Same as `ErrorMapper` but collected from `StreamMiddleware` values.

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
    Run(ctx context.Context, payload []byte) error
}
```

A single-mode message handler. `Keys()` declares which routing keys this handler subscribes to. Tag with `scribus.CommandTag`.

### BatchHandler

```go
type BatchHandler interface {
    Run(ctx context.Context, keys []string, payloads [][]byte) error
}
```

A batch-mode message handler. Receives a slice of messages flushed together. Provide exactly one without a tag.

### Consumer

```go
type Consumer interface {
    SetHandler(handler func(ctx context.Context, keys []string, documents [][]byte) error) error
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
