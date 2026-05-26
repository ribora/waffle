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
| _blank import_ `_ "path"` | Aggregate `di.Wire` / `di.Define` calls from another package (see below) |

### Modules — aggregating registrations from another package

A blank import (`import _ "path"`) tells Diogen to load the referenced package's source and merge the `di.Wire` / `di.Define` calls found there into your own configuration. Each waffle tool ships an `InitXxx()` function whose body is a set of DI registrations — blank-importing the tool's config package lets you pull those registrations in by import path instead of copying them by hand.

The convention is to keep all module includes in a dedicated `modules.go` file (a sibling of `di.go`), containing **only** the import block:

```go
// config/di/modules.go
package di

import (
    _ "gitlab.com/gorib/waffle/config/app"
    _ "gitlab.com/gorib/waffle/config/tools/http"
    _ "gitlab.com/gorib/waffle/config/tools/grpc"
    _ "gitlab.com/gorib/waffle/config/tools/migrate"
)
```

This keeps the high-level service composition visually separate from the imperative `di.Wire` calls in `di.go`.

Three namespaces ship with waffle:

- `waffle/config/app` — the application root. Includes one registration: `app.App` wired from `app.Init` collecting all commands tagged `app.CommandTag`. Always present in your `modules.go`.
- `waffle/config/service/*` — infrastructure modules (`pry` logger, `hilton` observability, `sql` db, `redis`, `elastic`).
- `waffle/config/tools/*` — tool registrations (each pulls a `Command` into the app, e.g. `http`, `grpc`, `migrate`, `scribus/single`, `scribus/batch`).

Properties:

- Only blank imports are scanned. Regular imports (used for type references in your code) are *not* treated as DI modules — if you need both, declare both: a regular import for the type usage and a blank import for the DI scan.
- The included package's whole AST is scanned for `di.Wire` / `di.Define` (it doesn't matter that they sit inside an unused `InitXxx()` function — Diogen never calls it)
- Includes are **transitive**: A blank-imports B, B blank-imports C — all three are merged
- **Cycles** (A → B → A) are caught by the Go compiler as a real import cycle
- **Tags match across packages**: a tool registering `di.Tag(MiddlewareTag)` and your code registering under the same constant are collected together

Constraints:

- Conditional wiring (`if env := ... { di.Wire[...](...) }`) inside an included package may fail to compile if it references local variables — the generator copies expressions as-is. Keep conditions in your own `InitDi()`.

## The InitXxx functions

Each waffle tool ships an `InitXxx()` function (e.g. `InitHttp()`, `InitGrpc()`) whose body is a set of `di.Wire` / `di.Define` calls. These functions are never called at runtime — Diogen reads their bodies directly when you blank-import the surrounding package.

For example, `config/tools/http/init.go`:

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

To use the HTTP tool, blank-import its config package from `config/di/modules.go`:

```go
// config/di/modules.go
package di

import (
    _ "gitlab.com/gorib/waffle/config/app"
    _ "gitlab.com/gorib/waffle/config/tools/http"
    _ "gitlab.com/gorib/waffle/config/tools/grpc"
    // ... other tool modules ...
)
```

Diogen scans every blank-imported package's AST for `di.Wire` / `di.Define` calls and merges them into the generated container. You don't reference the tool package directly anywhere — the blank import is the entire wiring contract. See [Modules](#modules--aggregating-registrations-from-another-package) above for the full semantics.

Each tool's documentation page shows both forms — the short blank-import line and, for reference, the explicit registrations it expands to. See [di/tags.md](tags.md) for all available tags.

### Manual registration (alternative)

If you need finer control — for example, swapping a tool's command name or wrapping its constructor — copy the registrations from `InitXxx()` directly into your `InitDi()` instead of blank-importing the module. Both styles are supported in the same setup; you can blank-import some tools and hand-write others.

## Project layout

DI configuration lives in two directories:

```
config/
└── di/
    ├── modules.go    Blank imports of waffle modules — no other code.
    ├── di.go         Hand-written wiring. Contains //go:generate directive and InitDi().
    └── diogen/
        ├── di.go     Hand-written adapter. Calls InitApp() and wires the closer.
        └── di_gen.go Generated container. Do not edit.
```

### config/di/modules.go

Declares which waffle modules this service is composed of. Contains **only** blank imports — no other code, no other imports:

```go
// Composition of waffle modules used by this service.
// Each blank import contributes its di.* registrations to the generated
// container via diogen. Add or remove a line to change the set of modules.
package di

import (
    _ "gitlab.com/gorib/waffle/config/app"
    _ "gitlab.com/gorib/waffle/config/service/pry"
    _ "gitlab.com/gorib/waffle/config/service/hilton"
    _ "gitlab.com/gorib/waffle/config/tools/http"
    _ "gitlab.com/gorib/waffle/config/tools/grpc"
    _ "gitlab.com/gorib/waffle/config/tools/migrate"
)
```

Keeping the module composition in its own file separates the high-level "what this service is made of" question from the lower-level "how the service-specific pieces wire together" question.

### config/di/di.go

Holds the `//go:generate` directive and the `InitDi()` function with service-specific wiring:

```go
//go:generate go tool diogen -output config/diogen/di_gen.go -pkg diogen $GOFILE

package di

import (
    "gitlab.com/gorib/di"
    "gitlab.com/gorib/env"
    "gitlab.com/gorib/pry"
    "gitlab.com/gorib/sql"
)

func InitDi() {
    // Infrastructure
    di.Wire[pry.Logger](func() (pry.Logger, error) {
        return pry.New(env.Value("LOGLEVEL", "info"))
    })
    di.Wire[sql.Db](sql.NewDb, di.Args(map[string]any{
        "driver": env.NeedValue[string]("DB_DRIVER"),
        "dsn":    env.NeedValue[string]("DB_DSN"),
    }))

    // ... domain wiring ...
}
```

Module packages are referenced only via blank imports in `modules.go` — they don't appear in `di.go`. Diogen loads them at generation time and merges their registrations into the container.

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
