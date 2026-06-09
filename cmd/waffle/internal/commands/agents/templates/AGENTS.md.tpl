# AGENTS.md

Instructions for AI coding agents working in this repository. This file is
**managed by `waffle agents`** — hand edits are lost on regeneration.

**Run `waffle agents` after every waffle upgrade** (`go get
gitlab.com/gorib/waffle@latest`) — framework conventions change between
versions and a stale AGENTS.md will steer you toward deprecated patterns.

For service-specific instructions (business rules, custom commands, known
quirks), edit `CONTRIBUTING.md` instead — this file does not touch it.

## This service

- Module: `{{ .ModulePath }}`
- Framework: [gorib](https://gitlab.com/gorib) + [waffle](https://gitlab.com/gorib/waffle){{ if .Domains }}
- Domains: {{ range $i, $d := .Domains }}{{ if $i }}, {{ end }}`{{ $d }}`{{ end }}{{ end }}{{ if .Modules }}
- Active waffle modules: {{ range $i, $m := .Modules }}{{ if $i }}, {{ end }}`{{ $m }}`{{ end }}{{ end }}{{ if .EnvVars }}
- Env vars: {{ range $i, $e := .EnvVars }}{{ if $i }}, {{ end }}`{{ $e }}`{{ end }}{{ end }}

## Rule #1 — prefer CLI over hand-writing

For any scaffolding task, first check whether a `waffle` command does it. Only
hand-write files when no command matches.

| Want to do | Command |
|---|---|
| New service | `waffle init service <name>` |
| New domain (model + service + repository + migrations) | `waffle make:domain <domain>` |
| Add inbound gRPC | `waffle make:domain:grpc <domain>` |
| Add inbound HTTP | `waffle make:domain:http <domain>` |
| Add AMQP consumer | `waffle make:domain:scribus <domain> <event>` |
| Add secondary repository | `waffle make:domain:repository <domain> <entity>` |
| Add migration | `waffle make:domain:migration <domain> <name>` |
| Add CLI subcommand | `waffle make:domain:command <domain> <name>` |
| Add DTO converter (any path) | `waffle make:converter <path>` |
| Refresh AGENTS.md | `waffle agents` |

Each command is idempotent — existing files are NEVER overwritten. You can
re-run any command safely.

## Rule #2 — always run validation after edits

After any change (yours or human's), run **in this order**:

```sh
make generate    # diogen, goverter, whisk, etc. — recompiles all codegen
make audit       # golangci-lint
make             # tests + build
```

Never commit between steps. If any of the three fails, fix before proceeding.

`make generate` is mandatory — diogen reads the entire DI graph and may emit
errors that surface only at generation time (missing dependency, circular,
duplicate alias). Skipping it leaves the service in a broken state.

## Project layout

```
api/<service>/proto/<domain>/        Generated proto stubs (don't edit; edit .proto)
cmd/app/main.go                      Entry point (don't touch beyond init)
config/
  di/
    modules.go                       Blank imports of waffle modules ONLY
    di.go                            //go:generate diogen; InitDi() with service-specific wires
  diogen/
    di.go                            Hand-written adapter
    di_gen.go                        GENERATED — never edit
domain/<x>/
  model/                             Domain types (no external deps)
  service/                           Business logic + interfaces (depends on model only)
  adapter/                           Outbound (postgres, gRPC clients, HTTP clients)
    postgres/                        sql repository (uses stored<X> with Unwrap/Inserts)
    <name>.go                        Other outbound adapters (flat files in adapter/)
  cmd/
    grpc/                            Inbound gRPC
      <domain>/service.go            Handler
      dto/convert.go                 goverter (see Rule #5)
    http/                            Inbound HTTP
      main.go                        Routes
      dto/convert.go                 goverter
    scribus/                         AMQP consumer (one per event)
      <event>.go                     Handler
      model/<event>.go               Event payload
      dto/convert.go                 goverter (single dto/ shared across events of a domain)
    migrate/                         Migrations source
      main.go                        NewMigrations() — embeds *.sql
      migrations/*.sql
pkg/
  goverter/convert.go                Project-wide conversion whitelist (see Rule #5)
```

## Rule #3 — naming conventions (strict)

These deviate from Go style guide but are project-wide. Don't "fix" them.

- `Db` not `DB`, `Dsn` not `DSN`, `Id` not `ID` (across all gorib + waffle + services)
- Constructor returns: prefer interface return (`func NewFoo() Foo`); if returning concrete struct, **export** it
- Domain interfaces declared by **consumer**, not producer:
  - gRPC handler declares `type FooManagement interface { ... }` — what it needs from service
  - Service implements it without referencing `grpc.FooManagement` — wired via `di.Alias[grpc.FooManagement]()`
- File naming: snake_case for files (`back_in_stock.go`), CamelCase for types

## Rule #4 — dependency injection (diogen)

DI is **compile-time**. There is no runtime container. Read `config/di/di.go`
carefully before touching it.

### Wiring primitives

```go
di.Wire[Interface](constructor, ...opts)        // bind concrete to interface
di.Define(constructor, ...opts)                  // bind without interface
di.Tag(SomeTag)                                  // mark for collection
di.Tags[T](SomeTag)                              // collect all values tagged
di.For[Consumer]()                               // scope binding to one consumer
di.Args(map[string]any{...})                     // override constructor args
di.Alias[OtherInterface]()                       // expose same instance under multiple ifaces
di.Fallback()                                    // low-priority default; dropped if a normal/Override provider exists
di.Override()                                    // win over normal/Fallback providers for the slot
```

Provider priority per slot: `Fallback` < normal < `Override`. Two providers at
the same tier for one slot (without `di.For` or mutually exclusive conditions)
is a generation error, not last-wins.

### Anti-patterns

- ❌ Manually calling constructors anywhere outside `InitDi()` or its delegates
- ❌ `func init()` for DI registration (use `di.Wire` in `InitDi()`)
- ❌ Adding entries to `di.go` for waffle tools — use **blank import** in `modules.go`
- ❌ `di.Include("path")` — DEPRECATED, use blank imports

### Adding a waffle module

Edit `config/di/modules.go` only:

```go
import (
    _ "gitlab.com/gorib/waffle/config/app"
    _ "gitlab.com/gorib/waffle/config/tools/grpc"
    _ "gitlab.com/gorib/waffle/config/tools/scribus/single"   // ← add line here
)
```

Then `make generate`. Don't `di.Include`. Don't touch `di.go` for waffle tools.

## Rule #5 — DTO conversion (goverter)

Every transport / adapter that converts between external and domain types
uses goverter via a **single wildcard extend** pointing at the project-wide
whitelist:

```go
// domain/<x>/cmd/grpc/dto/convert.go
package dto

//go:generate go tool goverter gen ./...

// goverter:converter
// goverter:output:file ./convert_goverter_gen.go
// goverter:ignoreUnexported
// goverter:output:format function
// goverter:extend {{ .ModulePath }}/pkg/goverter:.*
type Convert interface {
    ExampleToProto(*model.Example) *proto.Example
}
```

### How extends resolve

- `goverter:extend pkg:Name` treats `Name` as a regex. `.*` matches everything
- `pkg/goverter/convert.go` re-exports gorib conversion helpers as package-level vars
- Adding a project-wide helper = one line in `pkg/goverter/convert.go`
- Per-converter override: extra `// goverter:extend pkg:Specific` below wildcard (last wins)
- Per-field override: `// goverter:map FieldName FieldName | pkg.Func` on the method

### Anti-patterns

- ❌ Listing 7 `extend` lines instead of one wildcard
- ❌ Putting `goverter:extend` directives in goverter `interface` body (only as doc comments above `type Convert interface`)
- ❌ Trying to embed interfaces — goverter rejects `method must have one name`
- ❌ Hand-written convert.go when struct has ≥5 fields or nested types

## Rule #6 — errors

- `failure.Wrap(format, ...args)` / `failure.New(format, ...args)` — for errors that **cross service boundaries** (gRPC, AMQP, HTTP response bodies). Stable JSON wire format
- `errors.New("...")` / `fmt.Errorf("%w: ...", parent, ...)` — internal errors that don't leave the process
- Domain-level sentinel errors: `var ErrNotFound = errors.New("not found")`

## Rule #7 — observability

`hilton.Provider` is in ctx automatically — propagated by transport middleware
(`http.server/middleware`, `grpc.server/interceptor`, `amqp/interceptor`).

Custom span inside a handler:

```go
ctx, span := hilton.ProviderFromContext(ctx).
    Tracer("my-service").
    Start(ctx, "doWork", hilton.WithSpanKind(hilton.SpanKindInternal))
defer span.End()
```

Metrics: histogram only, no separate counters. `db.client.operation.duration_count`
already exists as `_count` aggregate — don't add `db.client.operations` counter.

## Rule #8 — common patterns by task

### Add a new gRPC method to existing domain

1. Edit `api/<service>/proto/<domain>/service.proto` — declare the RPC
2. `make generate` — regenerates proto stubs
3. Add method to `domain/<x>/cmd/grpc/<domain>/service.go`
4. Add Management interface method (if needed) — to the **handler's** interface
5. Implement on service in `domain/<x>/service/...`
6. Add DTO conversion in `domain/<x>/cmd/grpc/dto/convert.go`
7. `make generate && make audit && make`

### Add a new database table

1. `waffle make:domain:migration <domain> <name>`
2. Edit the `.sql` file in `domain/<x>/cmd/migrate/migrations/`
3. Add/edit `model.X` struct
4. Add/edit `stored<X>` in `domain/<x>/adapter/postgres/stored_<x>.go`
5. `make generate && make audit && make`

### Add an outbound HTTP client

1. Create `domain/<x>/adapter/<name>.go` by hand (small file — see `domain/<other>/adapter/<other>.go` for shape)
2. `waffle make:converter domain/<x>/adapter/<name>/dto` — DTO converter
3. Wire in `config/di/di.go`:
   ```go
   di.Wire[<x>service.<Name>Client](<x>adapter.New<Name>Service, di.Args(...))
   ```
4. `make generate && make audit && make`

### Add a new domain

```sh
waffle make:domain users               # model + service + postgres + migrations
waffle make:domain:grpc users          # if it needs gRPC
waffle make:domain:scribus users foo   # if it consumes AMQP events
```

Then edit business logic in `domain/users/service/users.go`.

## Rule #9 — what NOT to do

- ❌ `protoc` directly — `make generate` handles it via Buf
- ❌ Manual `go mod tidy` after every edit — `make .tidy` does it consistently
- ❌ Add `replace` directives in `go.mod` — coordinate gorib version bumps instead
- ❌ Touch `di_gen.go`, `convert_goverter_gen.go`, `*_goverter_gen.go`, `*_validation_gen.go`, `*_mock.go` — they're generated
- ❌ Wrap stdlib types in custom abstractions inside `domain/<x>/model/` — model stays pure Go types
- ❌ Pass `context.Background()` deeper than the entrypoint — propagate ctx from caller
- ❌ `panic(err)` outside `try.Catch`-wrapped blocks (and try is deprecated — prefer explicit error returns)

## Reference

- Waffle docs: `gitlab.com/gorib/waffle/-/blob/main/docs/`
- gorib core libs: `gitlab.com/gorib/<package>/README.md`
- DI overview: `gorib/di/README.md`
- DTO pattern: `waffle/docs/conventions/dto-conversion.md`

## Service-specific notes

For instructions specific to **this** service (business rules, custom commands,
known quirks), edit `CONTRIBUTING.md`. `waffle agents` does not touch it.
