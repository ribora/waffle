# Dependency Injection Overview

Waffle uses `gitlab.com/gorib/di` for dependency injection. All wiring is resolved at compile time — there is no reflection, no container lookup at runtime, and no magic. The DI container is a generated Go file that is re-created every time you run `make generate`.

## API

### di.Wire — wiring a constructor to an interface

```go
di.Wire[Interface](constructor, opts...)
```

Registers `constructor` as the provider for `Interface`. The generator resolves constructor arguments from other registered types.

```go
di.Wire[app.App](app.Init, di.Args(map[string]any{
    "commands": di.Tags[app.Command](app.CommandTag),
}))
```

### di.Define — registering a concrete constructor

```go
di.Define(constructor, opts...)
```

Registers a concrete type (no interface). Use this when the type is injected by its concrete type, or when registering tagged implementations collected into a slice.

```go
di.Define(heraldGrpc.NewNotificationService, di.Tag(grpcCommand.ServiceTag))
di.Define(heraldGrpc.NewErrorHandler, di.Tag(grpcCommand.MiddlewareTag))
```

Use `di.Alias` when the same concrete instance must satisfy multiple interfaces:

```go
di.Define(notificationService.NewNotificationManagement,
    di.Alias[scribusCommand.NotificationManagement](),
    di.Alias[heraldGrpc.ChannelManagement](),
    di.Alias[heraldGrpc.NotificationManagement](),
)
```

### Options

| Option | Description |
|--------|-------------|
| `di.Tag(tag)` | Register the result under a tag so it can be collected into a slice |
| `di.Tags[I](tag)` | Collect all values registered under `tag` as `[]I` — used as an argument value |
| `di.For[I]()` | Scope a registration to a specific interface (disambiguates when multiple `Wire` calls target the same interface) |
| `di.Args(map[string]any{})` | Provide literal argument values instead of resolved types |
| `di.Alias[I]()` | Expose the same concrete instance under an additional interface |
| `di.Defaults(map[int]any{})` | Default values for positional arguments |
| `di.Fallback()` | Mark as fallback — used only when no other registration matches |

## The InitXxx functions

Each waffle tool ships an `InitXxx()` function that shows **which `di.Wire` / `di.Define` calls are needed** for that tool. These functions are not meant to be called — they exist as reference implementations that you copy into your own `config/di/di.go`.

For example, `tools/http/init.go`:

```go
func InitHttp() {
    di.Wire[app.BaseCommand](
        func() app.BaseCommand { return app.NewCommand("http", "Start web server") },
        di.For[HttpCommand](),
    )
    di.Wire[MiddlewareList](NewMiddlewareList, di.Args(map[string]any{
        "middlewares": di.Tags[Middleware](MiddlewareTag),
    }))
    di.Wire[HttpCommand](NewHttpCommand, di.Tag(app.CommandTag))
}
```

To use the HTTP tool, copy these registrations into your `config/di/di.go`, adjusting package prefixes:

```go
//go:generate go tool diogen -output config/diogen/di_gen.go -pkg diogen $GOFILE

package di

import (
    "gitlab.com/gorib/di"
    "gitlab.com/gorib/waffle/app"
    whttp "gitlab.com/gorib/waffle/tools/http"
)

func InitDi() {
    // ... other registrations ...

    di.Wire[app.BaseCommand](
        func() app.BaseCommand { return app.NewCommand("http", "Start web server") },
        di.For[whttp.HttpCommand](),
    )
    di.Wire[whttp.MiddlewareList](whttp.NewMiddlewareList, di.Args(map[string]any{
        "middlewares": di.Tags[whttp.Middleware](whttp.MiddlewareTag),
    }))
    di.Wire[whttp.HttpCommand](whttp.NewHttpCommand, di.Tag(app.CommandTag))

    di.Wire[app.App](app.Init, di.Args(map[string]any{
        "commands": di.Tags[app.Command](app.CommandTag),
    }))
}
```

Each tool's documentation page shows the exact registrations to copy. See [di/tags.md](tags.md) for all available tags.

## Project layout

DI configuration lives in two directories:

```
config/
└── di/
    ├── di.go         Hand-written wiring. Contains //go:generate directive and InitDi().
    └── diogen/
        ├── di.go     Hand-written adapter. Calls InitApp() and wires the closer.
        └── di_gen.go Generated container. Do not edit.
```

### config/di/di.go

The single file where all DI registrations live. It contains the `//go:generate` directive that triggers code generation, and a single `InitDi()` function with all wiring calls:

```go
//go:generate go tool diogen -output config/diogen/di_gen.go -pkg diogen $GOFILE

package di

import (
    "gitlab.com/gorib/di"
    "gitlab.com/gorib/env"
    "gitlab.com/gorib/pry"
    "gitlab.com/gorib/storage/sql"
    "gitlab.com/gorib/waffle/app"
    grpcCommand "gitlab.com/gorib/waffle/tools/grpc"
)

func InitDi() {
    di.Wire[pry.Logger](func() (pry.Logger, error) {
        return pry.New(env.Value("LOGLEVEL", "info"))
    })
    di.Wire[sql.Db](sql.NewDb, di.Args(map[string]any{
        "driver": env.NeedValue[string]("DB_DRIVER"),
        "dsn":    env.NeedValue[string]("DB_DSN"),
    }))

    // ... domain wiring ...

    di.Wire[app.App](app.Init, di.Args(map[string]any{
        "commands": di.Tags[app.Command](app.CommandTag),
    }))
}
```

### config/diogen/di.go

A hand-written adapter that calls the generated `InitApp()` and wires the closer:

```go
package diogen

import "gitlab.com/gorib/waffle/app"

func InitDi() (app.App, error) {
    di, err := InitApp()
    if err != nil {
        return nil, err
    }
    application := di.AppApp()
    if closable, ok := application.(app.HasCloser); ok {
        closable.SetCloser(di.Shutdown())
    }
    return application, nil
}
```

### config/diogen/di_gen.go

Generated by `make generate`. Contains the `InitApp()` function and typed accessors. **Do not edit.**

## Registering your own types

### Interface

```go
di.Wire[service.UserRepositoryInterface](postgres.NewUserRepository)
```

### Concrete type with aliases

```go
di.Define(service.NewUserService,
    di.Alias[handler.UserManagement](),
    di.Alias[grpc.UserManagement](),
)
```

### Tagged (collected into a slice)

```go
// Register a gRPC handler
di.Define(usergrpc.NewUserGrpc, di.Tag(grpcCommand.ServiceTag))

// Register an HTTP middleware
di.Wire[http.Middleware](NewAuthMiddleware, di.Tag(http.MiddlewareTag))
```

## Code generation

`make generate` runs `diogen`, which reads the `//go:generate` directive in `config/di/di.go` and produces `config/diogen/di_gen.go`. The generated file contains an `InitApp()` function and typed accessors:

```go
// Generated — do not edit
func InitApp() (*Container, error) { ... }

type Container struct { ... }
func (c *Container) AppApp() app.App { ... }
func (c *Container) Shutdown() func() { ... }
```

The hand-written `config/diogen/di.go` calls `InitApp()` and exposes a clean `InitDi() (app.App, error)`.

### Automatic resource cleanup

`diogen` inspects every registered constructor at generation time. If the returned value implements `io.Closer`, it is automatically added to the shutdown list inside `InitApp()`. `Shutdown()` then calls `Close()` on all of them in **reverse initialization order** — so a database connection opened before a repository is closed after it.

You do not need to register closers manually. Implementing `io.Closer` is enough:

```go
type Store struct { db *sql.DB }

func (s *Store) Close() error { return s.db.Close() }
```

Anything that does **not** implement `io.Closer` is simply ignored during shutdown.

## Compile-time safety

Because the container is generated Go code, any misconfiguration surfaces as a compile error:

- Missing dependency → undefined symbol
- Wrong type → type mismatch at compile time
- Unresolvable constructor argument → generator error before compilation
