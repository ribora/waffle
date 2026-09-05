# AGENTS.md

Instructions for AI coding agents working in this repository. This file is
**managed by `waffle agents`** — hand edits are lost on regeneration.

**Run `waffle agents` after every waffle upgrade** (`go get
gitlab.com/gorib/waffle@latest`) — framework conventions change between
versions and a stale AGENTS.md will steer you toward deprecated patterns.

**Read `CONTRIBUTING.md` first, if it exists.** It holds service-specific
instructions — business rules, custom commands, known quirks — and it overrides
this file wherever the two disagree. `waffle agents` never touches it, so
service-specific notes go there and not here.

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
| New service | `waffle init:service` (prompts for the name) |
| New domain (model + service + repository + migrations) | `waffle make:domain <domain>` |
| Add inbound gRPC | `waffle make:domain:grpc <domain>` |
| Add inbound HTTP | `waffle make:domain:http <domain>` |
| Add AMQP consumer | `waffle make:domain:scribus <domain> <event>` |
| Add secondary repository | `waffle make:domain:repository <type> <name> <entity>` (`type` is `sql`) |
| Add migration | `waffle make:domain:migration <domain> <name>` |
| Add CLI subcommand | `waffle make:domain:command <domain> <name>` |
| Add DTO converter (any path) | `waffle make:converter <path>` |
| Refresh AGENTS.md | `waffle agents` |

Each command is idempotent — existing files are NEVER overwritten. You can
re-run any command safely.

Project conventions (API import path, proto directory, DTO mapper) live in
`waffle.yaml` at the project root — generators follow it instead of
hardcoded paths. See `waffle/docs/cli/overview.md#waffle.yaml`.

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
api/proto/<domain>/service.proto     Proto contract — edit this (path from waffle.yaml proto_dir)
api/pb/<domain>/                     Generated stubs — never edit (import path from api_import)
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
    grpc/                            Inbound gRPC (flat package, one per domain)
      service.go                     Handler: struct, Start(), RPC methods
      errors.go                      ErrorMapper (domain error → codes.Code)
      convert/                       goverter converter — only with mapper: goverter (see Rule #6)
    http/                            Inbound HTTP
      main.go                        Routes
      convert/                       goverter converter — only with mapper: goverter
    scribus/                         AMQP consumer (one file per event)
      <event>.go                     Handler
      model/<event>.go               Event payload
      convert/                       goverter converter — only with mapper: goverter (shared across a domain's events)
    migrate/                         Migrations source
      main.go                        NewMigrations() — embeds *.sql
      migrations/*.sql
pkg/
  goverter/convert.go                Project-wide conversion whitelist (see Rule #6)
```

The `mapper` key in `waffle.yaml` decides conversions. Default `goverter`: each
transport gets a `convert/` package as shown above. `dto`: no `convert/` package
is scaffolded — write conversions by hand (inline in the handler, or a `dto.go`
you add to its package).

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
- ❌ Taking a tagged type as a plain constructor argument. A `di.Tag`
  registration is a collection member and nothing else: it never fills a slot
  and never answers a single-value dependency. Collect with `di.Tags[T](tag)`,
  or drop the tag from the registration you meant to wire directly
- ❌ `di.Wire[T](someVar)` where `someVar` is a local. Only the value returned by
  `di.Define` is a handle — the generator cannot copy a local of `InitDi()` into
  the container. Register the constructor, or bind a handle first:
  `h := di.Define(NewT)`
- ❌ Registering the same constructor twice while both registrations are built
  (typically once with `di.Tag`, once without). Field names come from the type,
  and the constructor is the last thing left to tell two registrations apart —
  so there is no name for the second one

Each of these is a **generation error**, not a lint. They exist because the
output would otherwise compile and be wrong: a nil type assertion at startup, an
arbitrary member of a collection passed as a single dependency, or a constructor
run twice with the second value overwriting the first.

### Component tests — `Replacements`

The generated container has a second entry point for tests:

```go
func InitApp() (*Application, error)                        // = InitAppWith(Replacements{})
func InitAppWith(replacements Replacements) (*Application, error)
```

`Replacements` has one field per replaceable seam. Supply a value and the
constructor that would have produced it is never called:

```go
app, err := diogen.InitAppWith(diogen.Replacements{
    PostgresUserRepository: fakeUserRepository{},
})
```

The field's type is the interface the value is wired as, so a fake that stops
satisfying it fails to compile at the line that supplies it.

**It does not prune what fed that constructor** — anything eagerly built that
touches the environment or the network has to be in the cut too. See Rule #11,
"Steps and fixtures".

- ✅ Use `InitAppWith` for component tests that need the real graph and a few fakes
- ❌ Do **not** add `if env.Value("APP_ENV") == "test"` branches to `InitDi()` —
  that puts test knowledge into production wiring
- ❌ Do **not** use `di.Override()` to inject a mock — it is a build-time
  directive, so the mock has to be importable from production code

Not every registration has a field. A seam exists only when every type the
registration is exposed as is an interface: `di.Define(NewServer)` exposes the
concrete `*Server` and gets none, and `di.Tag` members get none because the
membership of a collection is fixed at generation time — replace the consumer
that collects them instead.

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

## Rule #5 — input parsing (whisk)

Anything the service accepts — HTTP body, query, header, AMQP delivery, a file —
is parsed by generated code. Put the tags on the struct, add one directive:

```go
//go:generate go tool whisk $GOFILE

type createOrder struct {
    Currency string   `scan:"query=currency" validate:"required,currency"`
    Email    string   `json:"email"          validate:"required,email"`
    Items    []string `json:"items"          validate:"required,unique,dive,required"`
}
```

A field with a `scan` tag comes from that scope; a field without one comes from
the body. Read it with `server.Into[T](src)`, or register the handler with
`server.Bind` to receive the parsed `*T` directly.

### Custom validators are a convention, not configuration

Write `Is` + the tag in PascalCase in any file under `pkg/validate` at the module
root — the lookup is by func name, not by file name:

```go
// pkg/validate/rules.go
package validate

func IsPasscode(value string, args ...string) bool { return len(value) == 6 }
```

No registration, no flag. A custom tag that shadows a built-in one wins, and
whisk warns when it does. The directory is `validate.storage` in `whisk.yaml`.

### Escape hatches

Sanctioned — reaching for one is not a failure of the generator:

| When | Hatch |
|---|---|
| A rule tags can't express — cross-field, business state | `func (s T) validate() error` |
| A field whisk can't convert, a derived value, a default | `func (s *T) parse(src scan.Source) error` |
| A body format that isn't JSON or XML | `Unmarshal<Format>` in `pkg/scan`, with `-format=<name>` |

Both hooks are lowercase methods you write by hand; whisk finds them.

### Anti-patterns

- ❌ Reading `src.Body()` and unmarshalling by hand in a handler
- ❌ Validating in the handler what a `validate` tag already expresses
- ❌ Registering a route with `server.Handle` when the DTO is typed — `Bind`
  captures the request and response types, and that record is what the OpenAPI
  generator reads; a `Handle` route stays out of the specification

## Rule #6 — DTO conversion

Default (`mapper: goverter`): each transport gets a generated `convert/`
package. It is the default because goverter **refuses to generate** on a field
it cannot map, so a field added to a model cannot silently stop reaching the
wire — a hand-written mapper compiles happily and drops it.

The generated package uses a **single wildcard extend** pointing at the
project-wide whitelist:

```go
// domain/<x>/cmd/grpc/convert/convert.go
package convert

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

## Rule #7 — errors

- `failure.Wrap(format, ...args)` / `failure.New(format, ...args)` — for errors that **cross service boundaries** (gRPC, AMQP, HTTP response bodies). Stable JSON wire format
- `errors.New("...")` / `fmt.Errorf("%w: ...", parent, ...)` — internal errors that don't leave the process
- Domain-level sentinel errors: `var ErrNotFound = errors.New("not found")`

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

## Rule #9 — common patterns by task

### Add a new gRPC method to existing domain

1. Edit `api/proto/<domain>/service.proto` — declare the RPC
2. Regenerate proto stubs (see Rule #10 — in-module vs. central)
3. Add method to `domain/<x>/cmd/grpc/service.go` (flat `grpc` package)
4. Add Management interface method (if needed) — to the **handler's** interface
5. Implement on service in `domain/<x>/service/...`
6. Add the proto ↔ domain conversion — in `domain/<x>/cmd/grpc/convert/convert.go` (default `mapper: goverter`), or by hand in the `grpc` package (`mapper: dto`)
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

## Rule #10 — what NOT to do

- ❌ `protoc` directly — proto generation is driven by the project's proto step (in-module: `go generate ./api/...`; monorepo with `proto_dir` outside the module: a central step such as `make -C <api-dir>`)
- ❌ Manual `go mod tidy` after every edit — `make .tidy` does it consistently
- ❌ Add `replace` directives in `go.mod` — coordinate gorib version bumps instead
- ❌ Touch `di_gen.go`, `*_goverter_gen.go`, `*_whisk_gen.go`, `*_store_gen.go`, `*_mock.go` — they're generated
- ❌ Wrap stdlib types in custom abstractions inside `domain/<x>/model/` — model stays pure Go types
- ❌ Pass `context.Background()` deeper than the entrypoint — propagate ctx from caller
- ❌ `panic(err)` outside `try.Catch`-wrapped blocks (and try is deprecated — prefer explicit error returns)

## Rule #11 — specification before code

### Which task this is

| task | what to do |
|---|---|
| a feature, or a **design defect** — an internal contradiction in behaviour that was specified | the full cycle below |
| a **code defect** at the level of a function — code that broke, not behaviour that was contradictory | a table test where the defect is, then the fix. None of the cycle below |
| a **spike** — the requirements are a sketch and the point is to find out what is possible | exempt from the cycle, but its output is a specification, not merged code. The spike's code is thrown away and rewritten through the cycle |

### 0. When the code came first

This service has behaviour nobody wrote down, so the first change to touch it
has nothing to sit inside. Derive a **baseline** from the code first: what it
does today, per declared contract, in the notation the cases will use.

A baseline read out of code inherits the code's defects, so **every case carries
a verdict**, and none is a boundary until it has one:

- **✓** confirmed as intended — a regression boundary.
- **✗** confirmed defect — the case that pins it asserts the *intended*
  behaviour and is therefore red until the fix lands.
- **?** no verdict, so not part of the boundary. Only the human who owns the
  behaviour moves it. Do not guess, and do not mark ✓ because the code is old.

**Adjudication is the human's.** Deriving cases is the cheap half. Expect to be
wrong about which are defects: on the two services this was built on, several
proposed defects were deliberate business rules and one could not be reproduced.

**When a defect is fixed the case flips ✗ → ✓** and its prose is rewritten to
describe the fixed behaviour, with one line saying what used to be wrong.

**Do not state counts you will not maintain.** "22 cases, 6 defects" in a header
is wrong within two edits, and a stale count reads as current.

### 1. Requirements in, specification out

Produce two artifacts and nothing else yet:

- **an executable specification** — a component test today, a `.feature` file
  once this service has a harness wired. Look in `tests/`; that is the fact,
  whatever any document says. See "Steps and fixtures". The **minimum** set that pins
  down what the requirements left open, not an exhaustive matrix.
- **a delta list** — every place the specification says something the source
  document does not, each pointing at the place it fills in. Include what was
  deliberately left **out** of scope: comparing two documents catches a wrong
  answer and misses a missing question.

The two go to different places. The specification is committed; the delta list
belongs with the ticket, because it is finished at approval. The reasoning for
each case is written **into** the specification.

For a **design defect** the deliverable is the set of cases that cannot all pass
at once. If the contradiction cannot be written as two cases, it is not yet
understood.

**The specification is one artifact, not prose plus code.** Do not produce a
Markdown description and an implementation beside it — that needs verifying
twice, and the second comparison is the one nobody does.

**One notation per service.** Two ways of writing cases in one repository is
load with no payoff. Follow whatever is already in `tests/`; if nothing is, the
choice is the human's, not yours.

**One specification per declared contract, not per service.** The
`interface{ A; B; C }` entry that `waffle make:tests:mocks --suggest` reports for
a shared container field *is* the list of contracts a domain owes — one
constituent per caller, each a place a scenario binds. Measured on one service:
a domain's management served four callers (gRPC, a consumer, two cron commands)
with no overlap at all, so a specification written against the gRPC operations
alone would have covered one contract in four.

### Writing the cases

**A case for a known defect asserts the behaviour you want, so it is red.**
Writing what the code currently does turns a defect into a green regression
boundary. Mechanical check before handing anything over: **the number of red
cases equals the number of known defects.** A mismatch means a defect got frozen
or a case is wrong; it caught both mistakes on the two services this was built
on.

**Fold duplicates along one axis only.** Same setup and same trigger, differing
only in the expectation, is one case with a column added. Differing in the
*setup* is not: merging leaves a column empty on most rows, and a sparse column
means two questions were forced into one table.

**Case titles describe behaviour, not identifiers.** `Create refuses an empty
code`, not `F2 Create refuses an empty code`. Reference numbers go in the file's
header comment.

**Language.** Step text and case titles are English — a closed vocabulary, half
of it wire identifiers. The **reasoning** — case comments and the delta list —
is written in the language of the requirements it is verified against, because
a translation inside that comparison is unaudited. Domain identifiers are never
translated: they are the names on the wire.

### 2. Verification, then approval

A human reads the source document and the delta list, then corrects or approves.
**No production code before approval.** If a case contains nothing that could
have gone another way, it did not need to be a case.

### 3. Steps and fixtures

**Bind at the declared contract** — the operation: a handler in `cmd`, a
consumer command, a cron command. Not the management interface below it: one
layer short skips conversion and the error→code map, and makes stand-ins invent
database behaviour (uniqueness, not-found) that belongs to the schema. Fixtures
go in through the container's own repositories, never through the operation under
test.

Then bind to the real container:

```go
app, err := diogen.InitAppWith(diogen.Replacements{ /* the cut */ })
```

**Do not build the infrastructure by hand.** The harness is a set of libraries:
`waffle/resource` is the three-method seam (`Provision`/`Migrate`/`Reset`, plus
an optional `Down`), `sql/sqltest` and `elastic/elastictest` are resources, and
`waffle/tools/grpc.Serve` puts the container's own handlers on an in-process
server chaining interceptors with the **same** `UnaryChain` the daemon uses.
What stays in this service is `Boot`, `Env` and the literal listing its
resources — `Boot` imports the generated container, so it cannot live elsewhere.
A run needs no configuration: with no DSN set, `sqltest.Docker` starts a postgres
container and the next run reuses it. See waffle's
`docs/conventions/component-harness.md`.

**Write a boot smoke test before any case**, in `tests/`, next to the scenarios:
it brings the resources up, boots on the intended cut and asserts only that the
callers are non-nil. One run then tells you what the cut is missing instead of an
hour inside a half-written suite, and it catches every eagerly-built node that
reaches for the environment or the network. (`waffle make:tests:component`
scaffolds a different, smaller check beside `InitAppWith` with no infrastructure
at all. Keep both.)

**Replacing a seam does not prune what feeds it.** `InitAppWith` is a flat eager
sequence: a node is skipped only if that node itself was supplied. Supplying a
typed gRPC client still builds the `ClientConnInterface` beneath it, and the boot
still demands that service's host and port. Replace **both** levels — the
connection so the boot stops reaching for the environment, the typed client
because that is the surface worth faking. Use `grpcClient.Dead(name)` for the
connection: reaching it panics, which means the replacement above did not take.

Cut at the **leaves**. `waffle make:tests:mocks --suggest` marks them `# leaf`:
their default constructor takes nothing else out of the container. Replacing an
interior seam deletes the subtree below it — the code the specification was
meant to exercise.

Three kinds of seam inside the cut, and the split is structural:

- one the test **programs** — an outbound client, an adapter — gets a generated
  mock from `mocks.generate`. Declare a negative expectation explicitly
  (`.Times(0)`); an assertion that exists only as a missing line cannot be
  reviewed.
- one that merely has to be **non-nil for the boot** gets a no-op owned by the
  harness, not the test. Use the one shipping with the interface — `pry.Noop()`,
  `grpcClient.Dead(name)` — rather than writing another.
- one whose calls a **`Then` step inspects** gets a **spy**: `pry.Recorder()`,
  `scribus.Recorder()`, `amqp.NewPublisherRecorder()`. Hand-write one only for a
  seam that is yours alone. A *stub* programs a return, a *mock* asserts
  expectations declared up front, a *spy* records calls for later — and a `Then`
  needs the third, because the expected value is often read from the database at
  assertion time and is unknown when the scenario is set up.

**A silent stand-in is only for a leaf the specification says nothing about.**
`Noop()` is a recorder nobody queries. A no-op consumer absorbs `SetExchange`,
`SetQueue` and `Bind`, so a typo in a handler's routing key stays green while
production receives nothing. When a step asserts on what a leaf was told, take
the recorder.

Three things the `--suggest` marker does not know, which is why a person chooses
the cut:

- `sql.Db` is a leaf, but the test hands it a real connection to a test
  database. It stays in `mocks.ignore`, and that entry is authoritative: a
  stand-in for a dependency carrying a language the code itself generates makes
  the test check the language rather than the behaviour.
- a factory that builds its clients from runtime data keeps the real boundary
  below the leaf and outside the container. A mock of that boundary cannot reach
  the app through `Replacements` at all; cut at the factory's own port instead.
- **controlled time is usually not needed.** Where the code compares a fixture's
  date against `time.Now()`, set the fixture relative to now
  (`now.AddDate(-1, 0, 0)` for "already started"). Reach for a clock seam only
  when a case needs time to *move* — and adding one is production code, which
  this step does not permit yet.

Still no production code at this stage.

### 4. Red-green

The specification and the steps are **frozen** at approval. The loop changes
production code and nothing else.

Never reach green by weakening an assertion, widening a matcher, deleting a case
or relaxing a fixture. If the test cannot go green without touching the
specification or the steps, stop — that is the next section, not a licence.

### Changing an approved specification

Only through the human who approved it, whatever the reason. **Never on your own
judgement.** Two triggers:

- an **internal logical conflict**: two approved cases cannot both hold. State
  the conflict and stop.
- an **external constraint**: the case needs data this service does not own, or
  an ordering the transport does not guarantee. State the constraint and stop.

The human decides what changes — the specification, the scope, or the design.

### What the merge request carries

The **specification**, as a diff, and a **link to the ticket** where the delta
list was argued. Approval itself is not machine-checkable: a pipeline can verify
that a link is present, not that anybody read anything.

## Reference

- Waffle docs: `gitlab.com/gorib/waffle/-/blob/main/docs/`
- gorib core libs: `gitlab.com/gorib/<package>/README.md`
- DI overview: `gorib/di/README.md`
- DTO pattern: `waffle/docs/conventions/dto-conversion.md`

## Service-specific notes

`CONTRIBUTING.md` holds instructions specific to **this** service — business
rules, custom commands, known quirks. `waffle agents` does not touch it, and a
service where this file is enough does not need one at all.

**It belongs to the humans who own the service. Do not create or edit it unless
you were asked to.** When you learn something that belongs there — a convention
you had to infer, a quirk that cost you a wrong turn — say so and quote what you
would write. Whether it gets written is theirs to decide.
