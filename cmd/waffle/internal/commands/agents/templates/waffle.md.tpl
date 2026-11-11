# waffle.md

The framework's conventions for this service, and what the service is made of.
`./waffle agents` writes this file, and an edit here is lost on the next run;
`AGENTS.md` beside it is the service's own.

**`make generate` rewrites it**, so the service context below — domains, active
modules, env vars — is as current as the container. After a waffle upgrade
(`go get gitlab.com/gorib/waffle@latest`) run it again for the conventions too:
they change between versions, and a stale copy steers you toward patterns that
are gone.

**Read `CONTRIBUTING.md` first, if it exists.** It holds service-specific
instructions — business rules, custom commands, known quirks — and it overrides
this file wherever the two disagree. `./waffle agents` never touches it, and it
belongs to the humans who own the service: when you learn something that belongs
there — a convention you had to infer, a quirk that cost you a wrong turn — quote
what you would write, and do not write it.

## This service

- Module: `{{ .ModulePath }}`
- Framework: [gorib](https://gitlab.com/gorib) + [waffle](https://gitlab.com/gorib/waffle){{ if .WaffleVersion }} `{{ .WaffleVersion }}`{{ end }}{{ if .Domains }}
- Domains: {{ range $i, $d := .Domains }}{{ if $i }}, {{ end }}`{{ $d }}`{{ end }}{{ end }}{{ if .Modules }}
- Active waffle modules: {{ range $i, $m := .Modules }}{{ if $i }}, {{ end }}`{{ $m }}`{{ end }}{{ end }}{{ if .EnvVars }}
- Env vars: {{ range $i, $e := .EnvVars }}{{ if $i }}, {{ end }}`{{ $e }}`{{ end }}{{ end }}

# Framework conventions

Reference for the framework itself. Read the entry you need; nothing below is a
protocol. The numbers are identifiers, not a reading order.

## Rule #1 — prefer CLI over hand-writing

For any scaffolding task, first check whether a `waffle` command does it. Only
hand-write files when no command matches.

**Run the service's own `./waffle`** — it is `go tool waffle` at the version in
`go.mod` — from the service's root, never a `waffle` found on `PATH`. The
commands are versioned with the service, and another version answers with other
rules.

Every command there is, so a gap in this table is a gap in the toolchain and not
something to fill in by hand without saying so. Operands and flags are in
`./waffle <command> --help`; a command that leaves work for you prints it:

| Want to do | Command |
|---|---|
| New service | `waffle init:service` |
| New domain (model + service + repository + migrations) | `./waffle make:domain <domain>` |
| Add inbound gRPC | `./waffle make:domain:grpc <domain>` |
| Add inbound HTTP | `./waffle make:domain:http <domain>` |
| Add event consumer | `./waffle make:domain:scribus --driver <id> <domain> <event>` |
| Add an entity to a domain | `./waffle make:domain:entity <domain> <entity>` |
| Set up migrations for a domain that has none | `./waffle make:domain:migrate <domain>` |
| Add one migration to a domain that already has them | `./waffle make:domain:migration <domain> <description>` |
| Add CLI subcommand | `./waffle make:domain:command <domain> <command>` |
| Make published events survive a crash after the commit | `./waffle make:outbox --driver amqp` |
| Add DTO converter (any path) | `./waffle make:converter <path>` |
| Refresh waffle.md | `./waffle agents` |

Outbound adapters are the hole: there is **no** command for a client of another
service or of a third-party API. Write it by hand, shaped like an existing one
in `domain/*/adapter/`, and wire it in `config/di/di.go` — see Rule #9.

Each `make:*` command is idempotent — existing files are NEVER overwritten — and
can be re-run safely, except `make:domain:migration`: its file name carries the
current second, so every run writes another migration.

Project conventions (API import path, proto directory, DTO mapper) live in
`waffle.yaml` at the project root — generators follow it instead of
hardcoded paths. See `docs/cli/overview.md` in waffle.

## Rule #2 — always run validation after edits

After any change (yours or human's), run **in this order**:

```sh
make generate    # diogen, goverter, whisk, etc. — recompiles all codegen
make audit       # golangci-lint
make             # .tidy, generate, build
```

Never commit between steps. If any of the three fails, fix before proceeding.

**And `make test` before you hand anything over** — once, not after every edit.
It is out of `make` on purpose: the component suite boots the real container
over real infrastructure, which is too slow for a loop that runs after every
change. It is also where `boundaries.Check()` lives, so the three commands above
passing does **not** mean the boundaries are clean. Run it, and paste what it
said.

`make generate` is mandatory — diogen reads the entire DI graph and may emit
errors that surface only at generation time (missing dependency, circular,
duplicate alias). Skipping it leaves the service in a broken state.

`go.mod` is never edited by hand. What `make .tidy` changes in it and in `go.sum`
is part of your change, not something to revert.

## Project layout

The scaffolds write it, so the shape is what `./waffle make:domain` left in
`domain/`. What the tree does not show:

- By convention `domain/<x>/model` depends on nothing and `service/` on `model/`
  only. What the check closes is `adapter/`, outbound and the only door to another
  domain (Rule #12), and `cmd/`, inbound
- `api/proto/` is edited; `api/pb/` and every `*_gen.go` are generated
- `config/di/di.go` is yours; `config/diogen/di_gen.go` is diogen's
- `mapper: dto` in `waffle.yaml` scaffolds no `convert/` package — conversions
  are written by hand

## Rule #12 — a domain is reached only through an adapter

`adapter/` is the one door out of a domain, and the neighbour's `model/` is the
one thing behind it. Only another domain's `adapter/**` (for `model/`), the
composition root and `tests/` may import a domain — `pkg/` may not, and domain
logic parked there is a boundary nobody can see. Inside a domain, `service/`
never imports its own `adapter/`: it declares what it needs and DI answers it.

The check reads `di.Wire`/`di.Define`/`di.Alias` as well as imports, so a
cross-domain wiring with no import is caught too. The full rule set is
`docs/architectures.md` in waffle.

The legal shape: the adapter declares the interface it needs (Rule #3), imports
the neighbour's model only for the types in that interface, and translates them
into its own.

```go
// domain/rent/adapter/invoice.go
package adapter

import (
	invoiceModel "{{ .ModulePath }}/domain/invoice/model"
	"{{ .ModulePath }}/domain/rent/model"
)

// Declared here, implemented by invoice/service, wired in config/di/di.go.
type InvoiceManagement interface {
	CreateInvoice(ctx context.Context, customerId int, amount money.Money) (*invoiceModel.Invoice, error)
}

func (s *invoiceService) toDomain(invoice *invoiceModel.Invoice) *model.Invoice { ... }
```

```go
// config/di/di.go — the neighbour's management answers the adapter's interface
di.Wire[rentAdapter.InvoiceManagement](invoiceManagementInstance)   // the only cross-domain shape
```

`invoiceManagementInstance` is the handle `./waffle make:domain invoice` published
in `config/di/di_invoice_domain.go`. A second `di.Define` of that constructor is an
ambiguous provider, and diogen takes one import name per package: `domain/<x>/service`
is `service`, as the scaffolds import it.

Behaviour crosses through the interface; types cross directly and stop at
`toDomain`. Past that line the neighbour's model does not appear.

`boundaries.Check()` runs in `make test`, not in the three commands of Rule #2.
There is no exceptions file: a crossing that would need one is a design
question for the platform engineer, not a line of configuration.

## Rule #3 — naming conventions (strict)

These deviate from Go style guide but are project-wide. Don't "fix" them.

- `Db` not `DB`, `Dsn` not `DSN`, `Id` not `ID` (across all gorib + waffle + services)
- **Constructors return the unexported struct, and the interface is on the
  registration.** `func NewUserRepository(...) *userRepository`, wired as
  `di.Wire[service.UserRepository](postgres.NewUserRepository)`, is what the
  repository scaffold writes. A handler has the same shape:
  `func NewUserHandler(...) *userHandler`, wired as `di.Wire[Routes](NewUserHandler)`
  because the router asks for a `Routes`. Exporting the struct only adds a second
  way to reach the same thing. Two exceptions: a constructor whose type is the
  interface (`func NewRouter(...) server.Router`), and a type collected by tag and
  reached no other way (`di.Define(..., di.Tag(...))`), where there is nothing to
  name it by
- Domain interfaces declared by **consumer**, not producer:
  - gRPC handler declares `type FooManagement interface { ... }` — what it needs from service
  - Service implements it without referencing `grpc.FooManagement` — wired by the
    management's handle, `di.Wire[fooGrpc.FooManagement](fooManagementInstance)`,
    which `./waffle make:domain:grpc <domain>` writes
- File naming: snake_case for files (`back_in_stock.go`), CamelCase for types

## Rule #4 — dependency injection (diogen)

DI is **compile-time**: `config/di/di.go` is read by diogen and turned into
`di_gen.go`; there is no runtime container. The primitives are in the diogen
README, and a mistake in wiring stops `make generate` with a message its
`docs/reference.md#error-catalogue` explains.

What generation does not stop, because it compiles and is wrong:

- ❌ Calling constructors by hand in production code, outside `InitDi()` or its
  delegates. A unit test does build its subject by hand
- ❌ `func init()` for DI registration — registrations go in `InitDi()`
- ❌ Registering a waffle module in `di.go` — it is a **blank import** in
  `config/di/modules.go`, then `make generate`
- ❌ `if env.Value("APP_ENV") == "test"` in `InitDi()` — test knowledge in
  production wiring. A component test substitutes through `Replacements`
- ❌ `di.Override()` to inject a mock — it is a build-time directive, so the mock
  would have to be importable from production code
- ❌ The AMQP publisher exposed as both `amqp.Publisher` and a service's port. Its
  `Replacements` field becomes a composite interface no recorder satisfies, so a
  component test builds the real publisher. Expose it as one type: the
  port alone, or, when `amqp.Publisher` is read too, the port declared as an alias,
  `type Publisher = amqp.Publisher`

**Supplying a `Replacements` field prunes what only that constructor fed.** Its
dependencies are not built, environment and network included, unless something
else the container builds still reads them.

## Rule #5 — input parsing (whisk)

Anything the service accepts — HTTP body, query, header, AMQP delivery, a file —
is parsed by code whisk generates from struct tags (`//go:generate go tool whisk
$GOFILE`). Tag syntax, custom validators and the hooks are in the whisk README.

**Before writing tags, run `go tool whisk tags`.** It prints every tag valid
here, built-in and this project's own. An invented tag fails generation; the
reverse — a check that already exists, written again by hand in a handler — is
caught by nothing.

- ❌ Reading `src.Body()` and unmarshalling by hand — `scan.Into[T](src)`, or
  `server.Bind` to receive the parsed `*T`
- ❌ `server.Handle` for a typed DTO — only a `Bind` route reaches the OpenAPI
  document

A rule tags cannot express goes in a hand-written `validate()` or `parse(src)`
method on the DTO. That is the sanctioned hatch, not a failure of the generator.

## Rule #6 — DTO conversion

Default (`mapper: goverter`): each transport gets a generated `convert/`
package. It is the default because goverter **refuses to generate** on a field
it cannot map, so a field added to a model cannot silently stop reaching the
wire — a hand-written mapper compiles happily and drops it.

`./waffle make:converter <path>` writes the `convert.go` with its directives
already on it — one method per type pair is all you add. Overrides and the
whitelist are in `docs/conventions/dto-conversion.md` in waffle.

- ❌ Listing `extend` lines one per helper — a single wildcard extend points at
  the project-wide whitelist in `pkg/goverter/convert.go`, and a new helper is
  one line there
- ❌ A hand-written `convert.go` when the struct has ≥5 fields or nested types

## Rule #7 — errors

- `failure.Wrap(format, ...args)` / `failure.New(format, ...args)` — for errors that **cross service boundaries** (gRPC, AMQP, HTTP response bodies). Stable JSON wire format
- `errors.New("...")` / `fmt.Errorf("%w: ...", parent, ...)` — internal errors that don't leave the process
- **Domain-level sentinel errors: `failure.New`, never `errors.New`.** This is
  what `./waffle make:domain` writes into `model/<entity>.go`, and the reason is the
  wire: `failure.New` registers a slug, and the slug is the `type` the HTTP
  error renderer puts in the body.

```go
// domain/user/model/user.go
var ErrUserNotFound = failure.New("user is not found")
// → {"code":500,"message":"user is not found","payload":{},"type":"user_is_not_found"}
```

A domain sentinel declared with `errors.New` reaches the client as
`"type":"internal_error"` — one label for every failure the service can have,
so no caller can tell them apart. A sentinel is domain-level precisely because
it does leave the process.

- **Throw a sentinel with `failure.Parse(ErrX)`**, not bare and not
  `failure.Wrap("%w", ErrX)`: `failure.New` holds no stack, and `Parse` returns
  a copy that carries one.
- **With context, the sentinel is the first `%w`:**
  `failure.Wrap("%w: {code}", ErrX, code)`, `failure.Wrap("%w: %w", ErrX, err)`.
  The type is taken from the first wrapped error, so `failure.Wrap("dsn: %w", err)`
  slugs the driver's message into an unbounded set of types.
- **A value goes into the format, not the sentinel.** `failure.New("role {role} exists")`
  works only while every caller passes the value; `failure.Wrap("%w: {role}", ErrX, name)`
  keeps the sentinel plain.
- **The cause travels with the error, on the wire too.** To keep it inside, log it and
  return the sentinel alone, as a repository does; never `fmt.Errorf`, which looks like
  wrapping and drops the cause from the reply without saying so.

## Rule #8 — observability

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

## Rule #9 — order of steps by task

Only tasks a command does not finish on its own — where the order, or what the
scaffold leaves for you, is the part that is not obvious. A task that is one
command is in Rule #1's table and not repeated here. Each of these ends with
Rule #2 — `make generate && make audit && make`.

| task | order |
|---|---|
| gRPC method on an existing domain | rpc in `api/proto/<x>/service.proto` → regenerate stubs (Rule #10) → method on `cmd/grpc/service.go` → the method on the **handler's** Management interface → implement it in `service/` → conversion in `cmd/grpc/convert/`, or by hand in the `grpc` package under `mapper: dto` |
| new database table | `./waffle make:domain:entity <domain> <entity>` writes the table with its model, repository, stored struct and DI → add the columns to the `.sql`, `model.X` and `stored<X>` → add the repository to the management, the parameter the command prints |
| change to an existing table | `./waffle make:domain:migration <domain> <description>` scaffolds the migration only → write the `.sql` → `model.X` → `stored<X>` in `adapter/postgres/stored_<x>.go` |
| outbound client — the one shape with no command | `adapter/<name>.go` by hand, shaped like an existing one → `./waffle make:converter domain/<x>/adapter/<name>/dto` → `di.Wire[service.<Name>Client](<x>Adapter.New<Name>Service, di.Args(...))` in `config/di/di.go`, with the interface declared by the consumer (Rule #3). A neighbour's management goes in by its handle: `di.Wire[<x>Adapter.<Name>Management](<name>ManagementInstance)` (Rule #12). Over HTTP the adapter takes its transport from the container — a `transport http.RoundTripper` parameter, `client.New(url, client.WithTransport(transport))` — wired `di.Wire[http.RoundTripper](client.NewTransport, di.For[service.<Name>Client]())`, one per neighbour, so a component test stands in for the neighbour under the client |

## Rule #10 — what NOT to do

- ❌ `protoc` directly — proto generation is driven by the project's proto step (in-module: `go generate ./api/...`; monorepo with `proto_dir` outside the module: a central step such as `make -C <api-dir>`)
- ❌ Manual `go mod tidy` after every edit — `make .tidy` does it consistently
- ❌ A `replace` in `go.mod` whose target is outside this repository — a local checkout of gorib or of anything else builds only on the machine it was written on. Release the module and bump the version instead; a `replace` onto another module of the same repository is fine
- ❌ Touch `di_gen.go`, `*_goverter_gen.go`, `*_whisk_gen.go`, `stored_*_gen.go` — they're generated
- ❌ Wrap stdlib types in custom abstractions inside `domain/<x>/model/` — model stays pure Go types
- ❌ Reach another domain other than through `adapter/`, or import `domain/<x>/cmd/**` from outside `cmd/` (Rule #12)
- ❌ Pass `context.Background()` deeper than the entrypoint — propagate ctx from caller
- ❌ `panic(err)` anywhere past the entrypoint — return the error

## Reference

Read the copy this service depends on, not the one on `main` — conventions
change between versions.

{{ if .WaffleVersion }}- Waffle docs: `$(go env GOMODCACHE)/gitlab.com/gorib/waffle@{{ .WaffleVersion }}/docs/`
{{ else }}- Waffle docs: `gitlab.com/gorib/waffle/-/blob/main/docs/`
{{ end }}
- Any other gorib module — diogen (`gitlab.com/gorib/di`), whisk, the core libs:
  `README.md` in the directory `go list -m -f '{{"{{"}}.Dir{{"}}"}}' gitlab.com/gorib/<module>` prints

