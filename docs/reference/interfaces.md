# Interface Reference

Every exported interface a service implements or is handed — waffle's own, plus
the ones from gorib packages it meets through waffle. The import path is in a
comment wherever it is not waffle's.

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

### Router

```go
// gitlab.com/gorib/http/server
type Router interface {
    Setup(mux Mux)
}
```

The one interface a service implements for every domain that serves HTTP, scaffolded by `waffle make:domain:http`. Register it under the collection — `di.Define(http.NewRouter, di.Tag(httpCommand.RouterTag))` — and every domain's routes land on one mux. `Setup` applies the middleware chain and registers routes; the same method is what the component harness calls, so a scenario exercises the routes that ship. See [tools/http](../tools/http.md#registering-routes).

### Mux

```go
// gitlab.com/gorib/http/server
type Mux interface {
    http.Handler

    Use(layers ...Layer)
    Group(prefix string, layers ...Layer) Mux

    Get(path string, layers ...Layer)
    Post(path string, layers ...Layer)
    Put(path string, layers ...Layer)
    Patch(path string, layers ...Layer)
    Delete(path string, layers ...Layer)
    Handle(method, path string, layers ...Layer)

    Routes() []RouteMeta
}
```

What `Setup` receives. chi-backed, so patterns are `{id}`, `{id:[0-9]+}` and a trailing `/*`. `Group` snapshots the layers and prefix at the call, after which parent and child diverge. `Routes()` is the route table the OpenAPI generator reads — which is why a route registered with `server.Bind` reaches the document and one registered with `server.Handle` does not, having no types to report.

### Handler and Layer

```go
// gitlab.com/gorib/http/server
type Handler func(src Source) (Renderer, error)
type Layer   = func(next Handler) Handler
```

Not interfaces, but the two shapes everything else composes around. A handler returning a nil renderer and a nil error renders 204.

`Layer` is a type **alias**, so a named function type — `middleware.Compress`, your own auth layer — is assignable without conversion. Middleware is therefore a `Layer` and not a tool-specific interface; the standard chain ships as `*middleware.Standard` and goes on with `mux.Use(standard.All()...)`. Its order is load-bearing at three points — see [tools/http](../tools/http.md#middleware).

`server.Handle` adapts a handler into a layer; `server.Bind` does the same for a typed `func(ctx, *Req) (*Response[Res], error)` and records the two types on the route.

### Source

```go
// gitlab.com/gorib/http/server
type Source interface {
    scan.Source // Lookup, LookupAll, Body, Context

    Path(key string) string
    Query(key string) string
    QueryAll(key string) []string
    Header(key string) string
    HeaderAll(key string) []string
    Cookie(key string) string
    Form(key string) string
    FormAll(key string) []string

    Request() *http.Request
    SetRequest(*http.Request)
}
```

The request, as an interface. You are handed one rather than implementing it. The embedded `scan.Source` is what lets a whisk-generated `Parse` read an HTTP request exactly as it reads an AMQP delivery; the typed accessors above it are for what has no generated `Parse` — middleware, and handlers written by hand. Query, form and body reads are cached, so a middleware and a handler cannot race for the body reader.

### Renderer

```go
// gitlab.com/gorib/http/server
type Renderer interface {
    Render(w http.ResponseWriter) error
    WriteBody(w io.Writer) error

    Status() int
    Headers() http.Header
    Body() any
    Stream() func(w io.Writer) error
}
```

What a handler returns. `*Response[Res]` implements it, built by `server.OK`, `Created`, `Accepted`, `NoContent`, `JSON`/`XML`/`Text`/`HTML`, or `StreamWith` for a body written incrementally. Implement it yourself only for a format none of those cover — the error middleware renders errors, so a handler never formats one.

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

Marker interface for the event consumer command, whatever driver it runs on.

### Handler

```go
type Handler interface {
    Keys() []string
    Run(src scan.Source) error
}
```

A single-mode message handler. `Keys()` declares which subjects this handler subscribes to. Tag with `scribus.CommandTag`. Parse the body with `scan.Into[T](src)`.

### BatchHandler

```go
type BatchHandler interface {
    Keys() []string
    Run(msgs []scan.Source) error
}
```

A batch-mode message handler. Receives a slice of `scan.Source` flushed together. Provide exactly one without a tag.

### Subjects

```go
type Subjects interface {
    Of(src scan.Source) string
    Match(pattern, subject string) bool
    Overlap(a, b string) bool
}
```

The routing semantics of the broker in use: where the subject sits on a source, what a pattern matches, and whether two patterns can claim the same message. Comes from the driver — `amqp.Subjects` for RabbitMQ — and is the reason scribus itself names no broker.

### Transport

```go
type Transport interface {
    Params() []string
    Apply(args []string) error
}
```

What the broker needs to be reached, named by the driver: `dsn exchange queue` for AMQP, `brokers topic group` for a Kafka driver, one queue URL for SQS. The command prints the names in `--help`, counts the operands against them and hands the values back in that order — it never names a connection parameter itself.

### Consumer

```go
type Consumer interface {
    Transport

    SetHandler(handler func(msgs []scan.Source) error) error
    Bind(subject string) error
    SetPageSize(pageSize uint16) error
    SetFlushInterval(flushInterval time.Duration) error
    SetReconnectInterval(interval time.Duration) error
    Consume(ctx context.Context) error
}
```

The consumer the driver supplies — `gitlab.com/gorib/amqp` for RabbitMQ. Configured and started by `scribusCommand.Run`, with `Bind` called once per subject a handler declares. Paging, flushing and reconnection are here because every broker has them; connecting is behind `Transport` because none of them spell it the same way. In production you do not implement this — it comes from the registrations in `config/tools/scribus/amqp/init.go` (see [Scribus Tool](../tools/scribus.md) for setup). A component test replaces it with `scribus.Recorder()`, which keeps that wiring and delivers through the handler the command installed (see [Component harness](../conventions/component-harness.md)).

---

## tools/outbox package

### OutboxCommand

```go
type OutboxCommand interface {
    app.Command
}
```

Marker interface for the relay command, whatever driver it runs on.

### Sender

```go
type Sender interface {
    Encode(document any) ([]byte, error)
    Send(ctx context.Context, subject string, payload []byte) error
}
```

The broker half, from a driver — `tools/outbox/rabbit` over `gitlab.com/gorib/amqp`. `Send` must return an error when the broker has not confirmed: the relay marks the row sent on its word, so a transport that reports success for a message the broker never took loses events silently. `Encode` is here rather than on the publisher because the payload is stored in whatever shape the driver will send, and encoded once. A component test replaces it with `outbox.Recorder()`.

### Store

```go
type Store interface {
    Add(ctx context.Context, entry Entry) error
    Claim(ctx context.Context, by string, staleAfter time.Duration) (*Entry, error)
    Sent(ctx context.Context, id string) error
    Failed(ctx context.Context, id string, cause error) error
    Purge(ctx context.Context, olderThan time.Duration) (int, error)
    Backlog(ctx context.Context) (Backlog, error)
    Reset(ctx context.Context, id string) (int, error)
}
```

The rows. `store.NewStore` over `gitlab.com/gorib/sql` is the one that ships. The requirement the signatures cannot express is that `Add` joins the transaction the context already carries — that is the whole mechanism, so a replacement runs [`outboxtest.Suite`](../../tools/outbox/outboxtest), whose first case catches an implementation that does not. `Claim` returns `nil, nil` when there is nothing to claim.

### Publisher

```go
type Publisher interface {
    Publish(ctx context.Context, subject string, document any) error
    Enqueue(ctx context.Context, subject string, document any) error
}
```

The seam a service points its own publisher interface at. The method name is the guarantee: `Publish` goes to the broker now, `Enqueue` writes a row in the caller's transaction. It deliberately has no `Close`, so it satisfies neither `Sender` nor `amqp.Publisher` — wiring it where one of those belongs would make the relay write rows instead of sending them, and that mistake does not compile.

### Relay

```go
type Relay interface {
    Run(ctx context.Context) error
    Close() error
}
```

Drains the outbox. Replace it to change how draining is paced; `outbox.NewRelay` takes the pacing as options.

---

## resource package

### Resource

```go
type Resource interface {
    Provision(ctx context.Context) error
    Migrate(ctx context.Context) error
    Reset(ctx context.Context) error
}
```

An external system a component test runs against for real. The package names no system and imports no client, so an implementation can live anywhere — `sql/sqltest`, `redis/redistest`, `elastic/elastictest`, or next to a client of your own. `Set` (from `resource.New`) runs `Up` once per run and `Reset` between scenarios. See [Component harness](../conventions/component-harness.md).

### Named

```go
type Named interface {
    Name() string
}
```

Optional. A resource with a name gets it quoted in errors, which matters once a suite lists several.

### Ephemeral

```go
type Ephemeral interface {
    Down(ctx context.Context) error
}
```

Optional. A resource with something to remove when the run ends — `sqltest.Ephemeral()` drops its database. Call `Set.Down` once per process, from `TestMain`.

---

## pry package

### Logger

```go
// gitlab.com/gorib/pry
type Logger interface {
    Trace(message any, opts ...Option)
    Debug(message any, opts ...Option)
    Info(message any, opts ...Option)
    Warn(message any, opts ...Option)
    Error(message any, opts ...Option)
}
```

What the framework hands every component, and what a handler or service takes as a constructor argument. `pry.Noop()` and `pry.Recorder()` both satisfy it, so a test needs nothing else. The options are `pry.Err(err)`, `pry.Ctx(ctx)` and `pry.Field(key, value)`; an `error` passed as the `message` wins over `Err` and leaves the message empty.

### Driver

```go
// gitlab.com/gorib/pry
type Driver interface {
    Log(Record)
}
```

Renders a record. `pry/zerolog` is the default and `pry/slog` the alternative; only the package you import links its library. The level gate, the fan-out to sinks and `Close` belong to the logger, which is why a driver is one method. A driver that also implements `io.Closer` is closed by `Logger.Close`.

`pry.Recorder()` is a driver — it sits under the real logger, so a scenario asserts on records the production path built.

### Sink

```go
// gitlab.com/gorib/pry
type Sink interface {
    Write(Record)
    Close() error
}
```

A side channel receiving every record the logger emits, off the calling goroutine. `channels.Sentry` is the one that ships. A sink sees only what the logger emits — the level gate runs before the record is built, so it cannot ask for anything below the logger's own level.

### Record

Not an interface, but the value both of the above receive:

```go
// gitlab.com/gorib/pry
type Record struct {
    Level   Level
    Message string
    Err     error
    Attrs   []Attr
    Ctx     context.Context
    Session *session.Session
}
```

`Session` is a copy taken when `pry.Ctx` ran. Its fields are all write-once, so the copy is safe to read from a sink's goroutine. The trace context is not among them — a sink reads that from `Ctx` via `session.TraceparentFromContext`, because it is rewritten per span.
