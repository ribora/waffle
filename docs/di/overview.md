# Dependency Injection Overview

[Docs](../README.md) · Dependency injection

Waffle uses `gitlab.com/gorib/di` for dependency injection. All wiring is resolved at compile time — there is no reflection, no container lookup at runtime, and no magic. The DI container is a generated Go file that is re-created every time you run `make generate`.

## API

### di.Wire — wiring a constructor to an interface

```go
di.Wire[Interface](constructor, opts...)
```

Registers `constructor` as the provider for `Interface`. The generator resolves constructor arguments from other registered types.

```go
di.Wire[app.App](app.Init, di.Args(map[string]any{
    "commands": di.Tags(app.CommandTag),
}))
```

### di.Define — registering a concrete constructor

```go
di.Define(constructor, opts...)
```

Registers a concrete type (no interface). Use this when the type is injected by its concrete type, or when registering tagged implementations collected into a slice.

```go
di.Define(notifyGrpc.NewNotificationService, di.Tag(grpcCommand.ServiceTag))
di.Define(notifyGrpc.NewErrorHandler, di.Tag(grpcCommand.ErrorMapperTag))
```

`di.Define` answers with a **handle**, and `di.Wire` takes one in place of a
constructor. That is how `waffle make:domain` splits a domain's wiring across
files: the management is defined once, and each transport wires the handle it
gets from the file next door.

```go
// config/di/di_user_domain.go
var userManagementInstance di.Handle

func InitUserDomain() {
    userManagementInstance = di.Define(userService.NewUserManagement)
}

// config/di/di_user_grpc.go
func InitUserGrpc() {
    di.Wire[userGrpc.UserManagement](userManagementInstance)
}
```

The handle carries the registration, not the value — passing anything else to
`di.Wire` is a generation error (see [What generation refuses](#what-generation-refuses-and-what-it-would-have-been)).
A handle that names another domain's package is checked like an alias: see the
[boundary rule](../architectures.md#the-boundary-rule).

Use `di.Alias` when the same concrete instance must satisfy multiple interfaces:

```go
di.Define(notificationService.NewNotificationManagement,
    di.Alias[scribusCommand.NotificationManagement](),
    di.Alias[notifyGrpc.ChannelManagement](),
    di.Alias[notifyGrpc.NotificationManagement](),
)
```

### Options

| Option | Description |
|--------|-------------|
| `di.Tag(tag)` | Register the result under a tag so it can be collected into a slice |
| `di.Tags(tag)` | Collect everything registered under `tag` — used as an argument value, and the parameter it fills says what they are collected as |
| `di.List(h1, h2)` | The same slice from named `di.Define` handles, in the order written — when the order matters and the membership is yours, not a tool's |
| `di.For[I]()` | Scope a registration to a specific interface (disambiguates when multiple `Wire` calls target the same interface) |
| `di.Args(map[string]any{})` | Provide literal argument values instead of resolved types |
| `di.Alias[I]()` | Expose the same concrete instance under an additional interface |
| `di.Fallback()` | Mark as a low-priority default — outranked by any normal or `Override` provider of the same slot, but still built in the branches a conditional one leaves empty |
| `di.Override()` | Mark as the winning provider for a slot — overrides normal and `Fallback` registrations (e.g. to replace a default pulled in from a blank-imported module) |
| _blank import_ `_ "path"` | Aggregate `di.Wire` / `di.Define` calls from another package (see below) |

**Provider priority.** When several registrations target the same slot (same interface, optionally narrowed by `di.For`), Diogen resolves them by priority: `di.Fallback()` < normal < `di.Override()`. The highest tier present wins; an unconditional lower-tier provider is still built wherever the winners' conditions leave the slot empty. Two providers at the same top tier whose conditions are not mutually exclusive are a **generation error** (ambiguous), not last-wins — disambiguate with `di.For`, mutually exclusive `if`/`else`, or by marking one `di.Override()` / the other `di.Fallback()`.

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

- `waffle/config/app` — the application root: the closing policy, the `di.Entry` and `app.App` wired from `app.Init` collecting all commands tagged `app.CommandTag`. Always present in your `modules.go`.
- `waffle/config/service/*` — infrastructure modules (`pry` logger, `hilton` observability, `sql` db, `redis`, `elastic`, `amqp` publisher).
- `waffle/config/tools/*` — tool registrations (each pulls a `Command` into the app, e.g. `http`, `grpc`, `migrate`, `scribus/single`, `scribus/batch`).

Properties:

- Only blank imports are scanned. Regular imports (used for type references in your code) are *not* treated as DI modules — if you need both, declare both: a regular import for the type usage and a blank import for the DI scan.
- The included package's whole AST is scanned for `di.Wire` / `di.Define` (it doesn't matter that they sit inside an unused `InitXxx()` function — Diogen never calls it)
- Includes are **transitive**: A blank-imports B, B blank-imports C — all three are merged
- **Cycles** (A → B → A) are caught by the Go compiler as a real import cycle
- **Tags match across packages**: a tool registering `di.Tag(ServiceTag)` and your code registering under the same constant are collected together

Constraints:

- A condition may use a variable its own `if` declares (`if cfg := config.Load(); cfg.Redis != nil`) — the container evaluates it once, under a name of its own. Any other **local variable** of the markup function, a `di.Handle` aside, is refused at generation: the generator copies expressions into the container and cannot copy the local. References to other *packages* are fine: their imports are carried into the generated file.

## The InitXxx functions

Each waffle tool ships an `InitXxx()` function (e.g. `InitHttp()`, `InitGrpc()`) whose body is a set of `di.Wire` / `di.Define` calls. These functions are never called at runtime — Diogen reads their bodies directly when you blank-import the surrounding package.

For example, `config/tools/http/init.go`:

```go
func InitHttp() {
    di.Wire[server.Mux](server.New)
    session := di.Define(middleware.NewSession)
    // ... the rest of the standard middlewares ...
    di.Wire[http.Global](http.NewGlobal, di.Fallback(), di.Args(map[string]any{
        "layers": di.List(compress, health, session /* ... */),
    }))
    di.Wire[app.BaseCommand](
        func() app.BaseCommand { return app.NewCommand("http", "Start web server") },
        di.For[http.HttpCommand](),
    )
    di.Wire[http.HttpCommand](http.NewHttpCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
        "routers": di.Tags(http.RouterTag),
    }))
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

To replace a single provider while still blank-importing the rest of a module, keep the blank import and register your replacement. A registration the module marks `di.Fallback()` loses to a plain one of yours; a plain one needs yours marked `di.Override()`, because two plain registrations of one slot are an ambiguity error. This works for single-provider slots; tagged collections are additive and can't be overridden this way.

`di.Override()` decides at generation time and belongs in the wiring. To swap an implementation **in a test**, use `Replacements` instead — see [Testing with the container](#testing-with-the-container).

## Project layout

DI configuration lives in two directories:

```
config/
├── di/
│   ├── modules.go    Blank imports of waffle modules — no other code.
│   └── di.go         Hand-written wiring. Contains //go:generate directive and InitDi().
└── diogen/
    ├── di.go         Hand-written adapter. Calls InitApp() and wires the closer.
    └── di_gen.go     Generated container. Do not edit.
```

They are siblings, not nested: `diogen` reads `config/di` and the `-output`
flag on the directive writes into `config/diogen`, so a generator never writes
inside the directory it reads.

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
    _ "gitlab.com/gorib/waffle/config/service/hilton/otel"
    _ "gitlab.com/gorib/waffle/config/tools/http"
    _ "gitlab.com/gorib/waffle/config/tools/grpc"
    _ "gitlab.com/gorib/waffle/config/tools/migrate"
)
```

Keeping the module composition in its own file separates the high-level "what this service is made of" question from the lower-level "how the service-specific pieces wire together" question.

### config/di/di.go

Holds the `//go:generate` directive and the `InitDi()` function with service-specific wiring:

```go
//go:generate go tool diogen -output config/diogen/di_gen.go $GOFILE

package di

import (
    "gitlab.com/gorib/di"
    "gitlab.com/gorib/env"
    "gitlab.com/gorib/pry"
    pryzerolog "gitlab.com/gorib/pry/zerolog"
    "gitlab.com/gorib/sql"
)

func InitDi() {
    // Infrastructure
    di.Wire[pry.Logger](func() (pry.Logger, error) {
        level, err := pry.ParseLevel(env.Value("LOGLEVEL", "info"))
        if err != nil {
            return nil, err
        }
        return pry.New(level, pryzerolog.New(pryzerolog.Plain())), nil
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

Generated by `make generate`. Contains the `Application` container, a getter per field, `Shutdown()`, and the two entry points `InitApp()` / `InitAppWith(Replacements)`. **Do not edit.**

## Registering your own types

### Interface

```go
di.Wire[service.UserRepositoryInterface](postgres.NewUserRepository)
```

### Concrete type with aliases

```go
di.Define(service.NewUserManagement,
    di.Alias[handler.UserManagement](),
    di.Alias[grpc.UserManagement](),
)
```

An alias that names another domain's package is the one place a cross-domain
dependency exists without an import, so it is checked: see the [boundary
rule](../architectures.md#the-boundary-rule).

### Tagged (collected into a slice)

```go
// Register a gRPC handler
di.Define(userGrpc.NewUserService, di.Tag(grpcCommand.ServiceTag))

// Register a gRPC error mapper
di.Define(userGrpc.NewErrorHandler, di.Tag(grpcCommand.ErrorMapperTag))
```

## Code generation

`make generate` runs `diogen`, which reads the `//go:generate` directive in `config/di/di.go` and produces `config/diogen/di_gen.go`:

```go
// Generated — do not edit
type Application struct { ... }

func (a *Application) AppApp() app.App { ... }
func (a *Application) Shutdown() func() error { ... }

type Replacements struct { ... }

func InitApp() (*Application, error)                       // = InitAppWith(Replacements{})
func InitAppWith(replacements Replacements) (*Application, error)
```

The hand-written `config/diogen/di.go` calls `InitApp()` and exposes a clean `InitDi() (app.App, error)`.

A parameter of a markup function — the initialization context, configuration fetched before the container exists — becomes a parameter of both, ahead of `replacements`: `InitApp(ctx context.Context, cfg *config.Config)`. The waffle modules declare none, so `InitApp()` stays bare until a service adds one; `di.go` then passes it, and so does a test that calls `InitAppWith`. See [Parameters](https://gitlab.com/gorib/di/-/blob/main/README.md#parameters) in the diogen README.

### Automatic resource cleanup

What `Shutdown()` closes is a closing policy declared in the markup, and `waffle/config/app` declares one: `di.DefaultCloser[io.Closer](di.Fallback())`. Every value it applies to is added to the shutdown list as it is built, and `Shutdown()` closes them in **reverse initialization order** — so a database connection opened before a repository is closed after it.

You do not register closers by hand. Implementing `io.Closer` is enough:

```go
type Store struct { db *sql.DB }

func (s *Store) Close() error { return s.db.Close() }
```

- A concrete type without a `Close` method is not closed. One whose `Close` has another signature is a generation error: say how to close it with `di.Close[I]()` on its registration.
- A value wired as an interface that does not declare `Close` is checked when it is built, and closed if it has the method.
- A service replaces the policy with a `di.DefaultCloser` of its own — see [Closing](https://gitlab.com/gorib/di/-/blob/main/docs/reference.md#closing) in the diogen reference.

A failed `InitApp()` closes them itself, in the same order: it returns no container, so nothing else could. A `Close()` that fails on the way out is joined onto the error that stopped the build rather than replacing it.

## Testing with the container

A component test runs the real graph and replaces only what leaves the process — the typed client of a neighbouring service, a publisher. The database and the repositories over it stay real. The generated `Replacements` struct is that seam: supply a value and the constructor that would have produced it is never called.

```go
type fakeContracts struct{ contract.ContractServiceClient }

func (fakeContracts) Get(ctx context.Context, in *contract.GetRequest, _ ...grpc.CallOption) (*contract.Contract, error) {
    return &contract.Contract{Id: in.Id}, nil
}

func TestUserFlow(t *testing.T) {
    container, err := diogen.InitAppWith(diogen.Replacements{
        ContractContractServiceClient: fakeContracts{},
    })
    if err != nil {
        t.Fatal(err)
    }
    defer container.Shutdown()()
    // ... exercise container.AppApp() or any getter ...
}
```

The connection under a replaced client is not built: a replacement prunes what only it fed. A client a test leaves real still needs its connection: fill that seam with `grpcClient.Dead(name)`, which panics naming the client if a call reaches it. Cut above anything that speaks a language the code generates: the typed client rather than its connection, and never `sql.Db`, which carries the SQL the repositories build.

**Infrastructure is a requirement.** The database and the repositories over it stay real, and an unreachable database fails the test loudly; it never skips. Fixtures go in through the container's repositories and assertions read back through them, so both run the code that ships.

**A unit test is the other shape.** It builds its subject by hand and uses no `Replacements`. There a mock is the default and a recorder the exception; in a scenario suite it is the reverse. waffle generates no mocks.

**`Noop()` is a recorder nobody queries**, so a scenario prefers a recorder: it costs the same, and the assertion is there when the scenario needs one.

| | |
|---|---|
| [`sql/sqltest`](https://gitlab.com/gorib/sql) | a real postgres: provisions, migrates and resets it |
| [`redis/redistest`](https://gitlab.com/gorib/redis) | the same for redis |
| [`elastic/elastictest`](https://gitlab.com/gorib/elastic) | the same for elasticsearch |
| [`pry.Recorder()`](https://gitlab.com/gorib/pry#testing) | a logger that keeps its records, under the code production runs |
| [`hilton.Recorder()`](https://gitlab.com/gorib/hilton#testing) | a provider that keeps spans and measurements |
| [`scribus.Recorder()`](../tools/scribus.md#testing) | replaces the consumer; `Deliver` runs the handler the command installed |
| [`amqp.NewPublisherRecorder()`](https://gitlab.com/gorib/amqp#testing) | replaces the publisher and keeps what was published |
| [`outbox.Recorder()`](../tools/outbox.md#testing) | replaces the `Sender`, so `Enqueue` still writes a row |
| [`grpcClient.Dead`](../tools/grpc.md#testing) | fills the connection under a client left real |

A `Replacements` field's type is the interface the value is wired as, so a fake that stops satisfying it fails to compile at the line that supplies it. Nothing is asserted at run time.

Which fields a test may supply is a separate question, and waffle leaves it to the suite.

Note that `InitDi()` in `config/diogen/di.go` calls `InitApp()`, which passes no replacements — production behaviour is unchanged. Tests call `InitAppWith` directly.

**Not every registration has an entry.** A `Replacements` field exists only where every type the registration is exposed as is an interface:

- `di.Define(NewServer)` exposes the concrete `*Server` — declare an interface and wire it as that if you want the slot replaceable.
- `di.Tag` members have no entry of their own: the membership of a collection is fixed at generation time, so replacing one member cannot change the set. But the collection its `di.Tags` consumer gathers has its own entry — a `Replacements` field of type `[]T` — so you replace the whole set, the HTTP routers say, without touching their consumer. A `di.List` gets the same entry, which is how the whole `[]server.Interceptor` chain is replaced. A member's own constructor is skipped only if it feeds nothing but that one collection; a member feeding more than one collection always builds for real regardless of what you supply.

Registrations that write the same container field — the two branches of an `if`/`else` over an env var, for instance — share a single entry, because which of them wins is decided at run time. Supplying it skips both branches.

See the [diogen reference](https://gitlab.com/gorib/di/-/blob/main/docs/reference.md#replacements) for the full rules.

## What generation refuses, and what it would have been

The generator's error messages are the visible half of the value — the other
half is the failure that did not happen. Every row below is a case the generator
refuses to build, with the behaviour you would have got instead. All eight are
covered by tests in the diogen repository.

| Generation fails with | Without the check |
|---|---|
| `dependency for db (Db) in *pkg.Store is not found` | does not compile — the constructor call has a missing argument |
| `circular dependency: A → B → A` | does not compile, or a stack overflow at startup with a reflection container |
| `ambiguous providers for Cache: NewMemory (…), NewRedis (…)` | one of them is picked silently; which one depends on registration order |
| `ambiguous di.Override providers for Cache` | as above, but between two registrations that both claim to win |
| `pkg.NewRules cannot be exposed as map[string]pkg.Rule` | the container is written with a field name derived from a composite type and dies inside `format.Source`, in a message that never mentions `di` |
| `di.Wire[Cache](c): c is not a handle returned by di.Define` | **compiles**, then `InitApp` always returns `interface conversion: interface is nil, not pkg.Cache` |
| `dependency for mw (Interceptor) in *pkg.Audit is provided only by tagged registrations (…)` | **compiles**, and one member of the collection is passed as the single dependency — chosen arbitrarily |
| `pkg.NewTracing is registered twice and both registrations are built` | **compiles**, the constructor runs twice, both values go to `closers`, and the collection reads the one that was written last |

The three marked **compiles** are the ones worth noticing. A build error is
caught by the compiler a second later — the generator adds nothing there. These
three produce output that builds and runs, and fails somewhere else: at startup,
or quietly, with the wrong value in a slice. No other layer would catch them.

That is the line the generator draws: output that does not compile is the
compiler's problem, output that compiles and is wrong is the generator's.

The full list of generator errors, each with the registrations that produce it,
is in the [diogen reference](https://gitlab.com/gorib/di/-/blob/main/docs/reference.md#error-catalogue).
For the other two generators on the path see
[input parsing](../conventions/input-parsing.md#troubleshooting-generation) (whisk)
and [make:domain](../cli/make-domain.md#troubleshooting-store) (store).

## Compile-time safety

Because the container is generated Go code, any misconfiguration surfaces as a compile error:

- Missing dependency → generation error (`dependency for … is not found`)
- Wrong type → type mismatch at compile time
- Unresolvable constructor argument → generator error before compilation
