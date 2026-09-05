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
- ❌ Touch `di_gen.go`, `*_goverter_gen.go`, `*_whisk_gen.go`, `*_store_gen.go`, `*_mock*.go` — they're generated
- ❌ Wrap stdlib types in custom abstractions inside `domain/<x>/model/` — model stays pure Go types
- ❌ Pass `context.Background()` deeper than the entrypoint — propagate ctx from caller
- ❌ `panic(err)` outside `try.Catch`-wrapped blocks (and try is deprecated — prefer explicit error returns)

## Rule #11 — specification before code

### Who decides what

Two roles, and this rule is written for the first one.

| | owns | in this repository |
|---|---|---|
| the person writing the specification | the cases, the steps, the red-green loop | `tests/feature/` |
| the person who owns the platform | what may be replaced, and the harness | `waffle.yaml`, `tests/internal/` |

**You work for the first.** The platform is a given: read it, use it, and when it
does not fit the case, say so and stop. Changing a seam to make a case pass
substitutes your judgement for the platform owner's, in the one place where the
whole suite's meaning is fixed.

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
`interface{ A; B; C }` entry that `waffle check:tests:replacements --suggest` reports for
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

**Name what the case excludes.** For a case of the form "X comes from Y", answer
which implementation would also pass it and should not. If one exists, the case
does not discriminate, and it needs variation — two facilities configured
differently rather than one. Put the answer in the delta list: a reviewer can
judge a named alternative, but not imagine the whole space of them.

Measured: three cases pinning "the adapter is asked with the facility's own
config" used one facility with one provider, so an implementation holding a
single configuration for the whole process passed all three. The second facility
is what made them cases.

**Fold duplicates along one axis only.** Same setup and same trigger, differing
only in the expectation, is one case with a column added. Differing in the
*setup* is not: merging leaves a column empty on most rows, and a sparse column
means two questions were forced into one table.

The same holds when both cases are real. Two cases that differ in one cell and
carry different information are still one table — written as prose they read as
the same case twice, and a reviewer spends the attention on telling them apart
instead of on what they say. Every column of the table has to be substituted
somewhere: a column nobody reads is a placeholder someone forgot to wire, and
the rationale it wanted to carry belongs in the comment above, naming the rows
in order.

**A case must not quietly re-assert a rule that has its own case.** Leaving a
field out of a request and then asserting the default it acquired tests the
default rule for a second time, in a case about something else — so when the
default changes, an unrelated case fails. Fill the cell.

**State the contract facts your cases lean on.** A case whose outcome depends on
something the contract does and no case says — that writing the same key again
edits rather than refuses, that a version is identified by a date — smuggles
that fact in. The reader cannot tell whether it is a rule or an accident, and
the reviewer cannot approve it. Either give it a case of its own or, when
another case already implies it, say it in the rule's comment.

**Assert the meaning, not the representation.** "The contract holds version 1"
pins how a link is stored; "the contract holds terms at 50 percent" pins what
has to survive, and it fails for the right reason when a later version is used
instead. Where both are available, the second is the case and the first is an
implementation detail the specification should not fix.

**Nothing here refers to the baseline.** It is a working artifact and is not
committed, so a reference to it or to its numbering points at a file the reader
does not have. Titles describe behaviour — `Create refuses an empty code`, not
`F2 Create refuses an empty code` — and a comment carries the reason itself
rather than a pointer to where the reason was argued.

**Language.** Step text and case titles are English — a closed vocabulary, half
of it wire identifiers. The **reasoning** — case comments and the delta list —
is written in the language of the requirements it is verified against, because
a translation inside that comparison is unaudited. Domain identifiers are never
translated: they are the names on the wire.

### 2. Verification, then approval

**Say that the arithmetic was read, not run.** Every number in the cases was
computed by reading the code, because the steps do not exist yet, and at that
volume one of them will be wrong. Saying so at the gate turns the correction
that follows the first run into an expected event instead of a surprise, and it
tells the reviewer which parts of the document carry the least confidence.
Measured: one row of a hundred and four, and the code was right.

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

#### What may be replaced is given, not chosen

`waffle.yaml` says what a test may supply itself, and it is **read-only for this
task**:

```yaml
replacements:
  allow:
    <import path>: [<interface>]
```

A permission list, not a list of exceptions: what it does not name may not be
replaced, in any form — mock, spy, fake or no-op. `waffle
check:tests:replacements` reads the `Replacements` literals in the tests and
fails naming any field the list does not cover, so this is not a convention you
can drift past. Do not add an entry to it. `--suggest` prints everything the
containers expose, granted or not; it is a menu for whoever owns the platform,
not a licence.

`tests/internal/harness/` is read-only too. `Boot` supplies what is a condition
of loading rather than a choice — the database, the logger, a dead connection
under a replaced client. Do not change it, and do not move a replacement in
there because a scenario was inconvenient.

**The form of the replacement is yours**, because it follows from the assertion:

- a **spy** — `pry.Recorder()`, `scribus.Recorder()`,
  `amqp.NewPublisherRecorder()`, or a hand-written fake that keeps what it was
  given — is read in a `Then`. A Gherkin step asserts after the call, which is
  the shape a spy answers.
- a **mock** (`mockgen`, `.EXPECT()`) declares before the call, so it fits where
  the expectation belongs to the `Given`. Declare a negative expectation
  explicitly (`.Times(0)`): an assertion that exists only as a missing line
  cannot be reviewed.
- a **no-op** (`pry.Noop()`, `grpcClient.Dead(name)`) keeps nothing, so no `Then`
  can read it.

The reasoning behind a particular grant is in waffle's
`docs/conventions/component-harness.md`. Read it to understand what you were
given; it is not a licence to change it.

#### When the cut does not work, stop

Say what you hit and what the specification needs, and hand it back. Do not work
around it. The recognisable cases:

- **a case needs a seam `replacements.allow` does not grant.** Name the
  interface and the assertion it would carry. Adding the grant is not yours.
- **a `Then` cannot see what it must assert** — the seam is a no-op, or a mock
  whose expectation would have to be declared before the scenario knows the
  value.
- **the boot reaches for the environment or the network.** `InitAppWith` is a
  flat eager sequence: a node is skipped only if that node itself was supplied,
  so a replaced typed client still has its connection built beneath it. If the
  harness has not covered that, it is a gap in the harness.
- **a case needs a seam that does not exist** — controlled time is the usual
  one. Where the code compares a fixture's date against `time.Now()`, set the
  fixture relative to now (`now.AddDate(-1, 0, 0)` for "already started") and no
  seam is needed. A case that needs time to *move* does need one, and adding it
  is production code, which this step does not permit.
- **the cut removes what the case was meant to exercise** — a repository
  replaced by a mock makes the test check the mock's SQL, not the schema's.

**Do not build the infrastructure by hand.** The harness is a set of libraries:
`waffle/resource` is the three-method seam (`Provision`/`Migrate`/`Reset`, plus
an optional `Down`), `sql/sqltest` and `elastic/elastictest` are resources, and
`waffle/tools/grpc.Serve` puts the container's own handlers on an in-process
server chaining interceptors with the **same** `UnaryChain` the daemon uses. A
run needs no configuration: with no DSN set, `sqltest.Docker` starts a postgres
container and the next run reuses it.

**Write a boot smoke test before any case**, in `tests/`, next to the scenarios:
it brings the resources up, boots on the given cut and asserts only that the
callers are non-nil. One run then tells you what the cut is missing instead of an
hour inside a half-written suite. (`waffle make:tests:component` scaffolds a
different, smaller check beside `InitAppWith` with no infrastructure at all.
Keep both.)

Still no production code at this stage.

### 4. Red-green

The specification and the steps are **frozen** at approval. The loop changes
production code and nothing else.

Never reach green by weakening an assertion, widening a matcher, deleting a case
or relaxing a fixture. If the test cannot go green without touching the
specification or the steps, stop — that is the next section, not a licence.

**A case that was never red has not been shown to fail.** It happens when the
fix and the case land together — a value moved into configuration with the new
number as its default, a condition widened — and the case is green from its
first run. Prove it: put the old value back for one run, check that exactly that
case fails and for its own reason, then restore. Say at the gate that you will,
and say afterwards that you did. A case nobody has seen fail is a decoration,
and the red count check above cannot notice it.

**A step can be the thing that is wrong.** A failure that is not on the expected
list is as likely to be a step that does not send what the case says as a defect
in the service — a column added to a table and not read, a value formatted for
the wire the wrong way. Read the failure before believing it: a step defect that
survives makes the case green while it tests nothing. Measured: one of sixteen.

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
