# CLI Overview

[Docs](../README.md) · CLI scaffolding

The `waffle` CLI is a standalone scaffolding tool. It generates the boilerplate for new services and domains so you can focus on business logic from the first commit.

## Install

```sh
go install gitlab.com/gorib/waffle/cmd/waffle@latest
```

## Commands

| Command | Description |
|---------|-------------|
| `waffle init:service` | Bootstrap a new microservice project |
| `waffle make:domain <domain>` | Add a domain (model, service, postgres adapter) |
| `waffle make:domain:grpc <domain>` | Add a gRPC handler for an existing domain |
| `waffle make:domain:http <domain>` | Add an HTTP handler for an existing domain |
| `waffle make:domain:scribus --driver <id> [--batch] <domain> <event> [subject]` | Add an event consumer for an existing domain; `--driver` names the broker module under `config/tools/scribus` (`amqp` today), `--batch` scaffolds a `BatchHandler` instead of a per-message `Handler` |
| `waffle make:domain:migrate <domain>` | Add database migrations for an existing domain |
| `waffle make:domain:entity <domain> <entity>` | Add an entity to an existing domain — model, repository interface, adapter, table and DI, the same set the domain's own entity gets. `--storage` picks the adapter and defaults to `sql`, the only one there is; an entity always gets a repository |
| `waffle make:domain:migration <domain> <description>` | Add a new SQL migration |
| `waffle make:domain:command <domain> <command>` | Add a CLI subcommand scoped to a domain, registered as `<domain>:<command>` |
| `waffle make:converter <path>` | Scaffold a goverter convert.go at the given path |
| `waffle make:outbox --driver <id>` | Enable the transactional outbox: the table, its migration runner, the module and the named broker driver under `config/tools/outbox`. See [tools/outbox](../tools/outbox.md) |
| `waffle tests:harness` | Regenerate the component harness from the container; the first run also scaffolds the suite ([details](../conventions/component-harness.md)) |
| `waffle tests:replacements [check] [--container=<path>]` | List everything the containers expose and what `replacements.allow` grants; `check` fails if a test replaces a field the list does not name |
| `waffle flows record` | Write `tests/surface.yaml`: what the service publishes, consumes, serves and calls, from its code ([details](../conventions/flows.md#the-codes-record)) |
| `waffle flows [check] <root> <index.yaml>` | Assemble the flows of every service below `root` into an index and pages; `check` fails on a stale page and on what the services' records leave unclosed ([details](../conventions/flows.md#the-projects-flows)) |
| `waffle agents` | Regenerate `AGENTS.md` after a waffle upgrade ([details](agents.md)) |
| `waffle agents:guard <mode> <path>`, `waffle agents:mode <mode>`, `waffle agents:grant <mode>` | Rule #11's modes, areas and grants, for a tool that enforces them ([details](agents.md#modes-areas-and-grants)) |

`--help` on any of them prints its description and operands. A failing command
exits non-zero, so a `make` target that calls one stops.

### Names

A domain, entity or event name becomes a Go identifier — the domain generator
emits `var <name>ManagementInstance` and `Init<Name>Domain` — as well as a
directory and, for migrations, part of a SQL identifier. So it has to be a
lowercase Go identifier: a letter, then letters, digits or underscores, and not a
Go keyword. `user`, `order_item` and `user2` are fine; `user-profile`, `2fa`,
`User` and `type` are refused.

A name of several words is folded into each identifier it reaches, and kept as
written wherever it is not one. `waffle make:domain:entity warehouse unit_type`
writes `model/unit_type.go` and `adapter/postgres/stored_unit_type.go`, declaring
`UnitType` and `storedUnitType`, against a table called `unit_types`.

The check runs before anything is written, together with the check that
`config/di/modules.go` exists. A command that cannot finish leaves no files
behind — `waffle make:domain --help` used to scaffold a domain called `--help`
and stop halfway through.

Two operands are not identifiers and follow their own rules: the `<path>` of
`make:converter` has to stay inside the project, and the `<description>` of
`make:domain:migration` is one file-name segment, so it may hold dashes and dots
but not a separator.

### Command names

A command name is the third, because most of what it becomes is a word somebody
types. `waffle make:domain:command media sweep-otp` derives every role from the
one operand:

| Role | Spelling |
|---|---|
| the CLI token | `media:sweep-otp` |
| the directory | `domain/media/cmd/sweep-otp/` |
| the package | `sweepotp` |
| the identifiers | `NewSweepOtpCommand`, `sweepOtpCommand` |
| the registration | `config/di/di_media_sweep_otp.go`, `InitMediaSweepOtp` |

So it takes lowercase words joined by `-` or `_`: `recalc`, `sweep-otp` and
`pay_recurrent_fee` are accepted, `sweepOtp`, `2fa` and a trailing separator are
not. The package carries no separator because Go package names do not.

The token is prefixed with its domain, which is not decoration. The framework's
own commands are bare words — `grpc`, `http`, `migrate`, `cron`, `scribus`,
`outbox` — in the same registry, and a second command answering to one of them
fails `app.Init`: the binary refuses to start, taking every other command with it.

### Subjects

`make:domain:scribus` takes the subject after the event, because the event names
the generated Go type and a subject is usually dotted:

```sh
waffle make:domain:scribus --driver amqp user order_paid order.paid
```

The subject defaults to the event, so `waffle make:domain:scribus --driver amqp user ping`
binds `ping` — which is why the event alone is enough when it happens to fit both
jobs. Give a dotted event with no subject and the command says so rather than
emitting a type called `order.paidCommand`.

## How it works

### Project root detection

Every scaffolding command walks up the directory tree from the current working directory looking for a `go.mod` file. The module name in that file becomes the base import path used in all generated files. You can run `waffle` from any subdirectory of your project.

### Template rendering

Each command ships embedded Go templates. When you run a command, waffle renders each template with the parameters you provide (service name, domain name, timestamp) and writes the result to the target path.

**Idempotent writes:** If the target file already exists, waffle skips it without error. You can safely re-run commands; only missing files are created.

### Post-generation hooks

Some commands run additional steps after writing files:

- `init:service` — installs the `diogen` and `waffle` tools, then runs `make .tidy generate`
- `make:domain` — installs the `store` tool and enables `config/service/sql` and `config/tools/migrate` (unless `--no-repository`), then runs `make .tidy generate`
- `make:domain:entity` — the same tail as `make:domain`, for one more entity
- `make:domain:grpc` — enables `config/tools/grpc`, runs `go generate ./api/...`, then `make .tidy generate`; when `proto_dir` is outside the module both runs are skipped and the command prints the steps to do by hand
- `make:domain:http` — enables `config/tools/http`, installs `whisk` (and `goverter`, under the default mapper) and writes `whisk.yaml`, then runs `make .tidy generate`
- `make:domain:scribus` — enables `config/tools/scribus/<driver>` and the mode module, installs `whisk`, then runs `make .tidy generate`
- `make:domain:command` — runs `make .tidy generate`
- `make:domain:migrate` — enables `config/service/sql` and `config/tools/migrate`, and stops there
- `make:outbox` — enables `config/service/sql`, `config/tools/migrate`, `config/tools/outbox` and the driver named by `--driver`, then runs `make .tidy generate`

`make:domain:migrate`, `make:domain:migration`, `make:converter`,
`tests:harness` and `agents` run no `make`; run `make generate` yourself
afterwards. `make generate` also runs `tests:harness` again, through the
`go:generate` line in the harness package.

The `make generate` step re-runs code generators (DI container, stored struct mappers) so the project compiles immediately after scaffolding. It is idempotent — diogen regenerates on every run and rewrites a file only when its content changed — so running it again after any command settles the question.

### Template functions

Templates have access to three extra functions beyond the standard library:

| Function | Example |
|----------|---------|
| `plural` | `plural "user"` → `"users"` |
| `title` | `title "user_profile"` → `"UserProfile"` |
| `camel` | `camel "user_profile"` → `"userProfile"` |

## waffle.yaml

Project-level conventions live in `waffle.yaml`, next to `go.mod`. `init:service` creates it with the defaults below; generators read it instead of hardcoding paths.

| Key | Default | Meaning |
|-----|---------|---------|
| `api_import` | `<module>/api/pb` | Import path prefix for generated proto stubs |
| `proto_dir` | `api/proto` | Where `.proto` sources are written |
| `mapper` | `"goverter"` | `"goverter"` scaffolds a goverter `convert/convert.go`; `"dto"` scaffolds nothing extra (hand-written conversions) |
| `replacements.allow` | empty | Interfaces a test may supply on the container's `Replacements`, keyed by import path: `gitlab.com/gorib/sql: [Db]`. A permission list — an empty one allows nothing |
| `agents.areas` | `model`, `gomod` | Named sets of paths that Rule #11's modes open, or that the human grants on top of a mode; paths may leave the service. See [agents](agents.md#modes-areas-and-grants) |

`<service>` is the last path segment of the module name. A missing `waffle.yaml`, or one missing some keys, behaves exactly like a file containing only the defaults above — the two are indistinguishable to every command. An unknown key (e.g. a typo) is a hard error, and an invalid `mapper` value (anything other than `dto` or `goverter`) is a hard error.

The whisk generator keeps its own `whisk.yaml` beside it, on the same missing-is-defaults rule. `make:domain:http` and `make:domain:scribus` write it the first time they pull whisk in, and never overwrite one you have edited; the keys are documented in [whisk's README](https://gitlab.com/gorib/whisk#whiskyaml).

## Detailed pages

- [init:service](init-service.md)
- [make:domain](make-domain.md)
- [make:domain:http](make-domain-http.md)
- [make:domain:grpc](make-domain-grpc.md)
- [make:domain:migrate](make-domain-migrate.md)
- [agents](agents.md)

The remaining commands have no page of their own. They follow the same rules as
the ones above — project root detection, idempotent writes, `make generate`
afterwards — and what each produces is described where the produced thing is:

| Command | Read |
|---|---|
| `make:domain:entity` | [make:domain](make-domain.md#a-second-entity) — the same second list, once more |
| `make:domain:scribus` | [tools/scribus](../tools/scribus.md) |
| `make:domain:migration` | [tools/migrate](../tools/migrate.md#sql-file-format) |
| `make:domain:command` | [core/commands](../core/commands.md#writing-a-custom-command), and [Command names](#command-names) above |
| `make:converter` | [conventions/dto-conversion](../conventions/dto-conversion.md#cli-scaffolding) |
| `make:outbox` | [tools/outbox](../tools/outbox.md#setup) |
| `tests:harness` | [conventions/component-harness](../conventions/component-harness.md#the-generated-file-and-yours) |
| `tests:replacements` | [below](#testsreplacements), and [di/overview](../di/overview.md#testing-with-the-container) for the mechanics |
| `flows` | [conventions/flows](../conventions/flows.md) |

`tests:harness` takes no operands. Every run rewrites
`tests/internal/harness/harness_gen.go` from the container, and the first run —
the one that finds no `harness.go` beside it — also scaffolds the suite. It
works from the module root or from anywhere below it, so the `go:generate` line
in `harness.go` runs it after diogen on every `make generate`. Which files those
are, and what goes in the one you write, is
[conventions/component-harness](../conventions/component-harness.md#the-generated-file-and-yours).

Everything in the generated file comes from the container rather than from a
flag, because a flag would close a set the container already knows:

- **the systems.** A seam of type `gitlab.com/gorib/sql.Db` means postgres, and
  the harness gets `sqltest` plus every `domain/*/cmd/migrate` it finds, in name
  order; a `gitlab.com/gorib/redis.Db` seam means `redistest`. Without either —
  an API gateway, a proxy — the set is `resource.New()`, whose `Up`, `Reset` and
  `Down` do nothing.
- **the transports.** `NewGrpcCommand` in the container gives an in-process gRPC
  server, `NewHttpCommand` gives `waffleHttp.Serve` over the container's own mux,
  global chain and router collection, and a service that has both gets both
  behind one `Clients`. A container still passing a single `server.Router`, or
  none of the `http.Global` the chain now arrives as, is refused with the fix
  rather than read as a service that serves no HTTP.
- **the handlers, their clients and the interceptor order**, read off the
  daemon's own construction call; a handler's `Start` names the service it
  registers, and the typed client is the constructor protoc-gen-go-grpc puts
  beside it.
- **the dead connections.** Every `grpc.ClientConnInterface` seam becomes
  `grpcClient.Dead(...)`, labelled with the clients built on it and the
  variables it reads.
- **the environment.** A typed field of `Variables` for every variable read
  through `gitlab.com/gorib/env`, unless `Boot` replaces the constructor that
  reads it: a placeholder for a `Need*` read, the fallback for the rest.

A handler whose registration it cannot find is reported by name rather than
guessed at.

### tests:replacements

What a test can replace are the fields of the generated `Replacements` struct, so
that struct is what this command reads. The unit is the interface, not the field:
several fields of one type collapse to a single entry, and an instantiated
generic collapses to the generic interface it instantiates.

The two halves read the module differently, and on purpose:

| | how | why |
|---|---|---|
| the menu — what the containers expose | `go/parser` on the generated file alone | it is what you ask for when nothing builds yet, and it is the only way to learn what there is to request |
| `check` — what the tests replace | `go/types` over the whole module | a `Replacements` value arrives through a helper's return, a field of the scenario struct, an element of a slice; none of those resolves by name, and all of them are how a suite is really written |

`check` therefore refuses to answer on a module that does not type-check, rather
than reporting that nothing is replaced — and a service in that state fails
`go test` anyway. Loading the module costs about 0.4s, once per run, against a
build cache `go test` has already warmed.

The comparison itself lives in `gitlab.com/gorib/waffle/replacements`, so a
service runs it from its own suite through the scaffolded
`TestReplacementsStayInsideTheList`. That is where the enforcement belongs — `go
test ./...` is what people and pipelines already run — so this command is the
report layer, and the comparison is what you ask for by name.

| | |
|---|---|
| no operand | print everything the containers expose, marking `# leaf` and `# allowed` |
| `check` | fail if a test fills in a field `replacements.allow` does not grant |
| `--container=<path>` | read this generated file instead of discovering it; repeatable |

`replacements.allow` is a **permission list**, not a list of exceptions: an empty
one allows nothing, and what it does not name may not be replaced in any form —
mock, spy, hand-written fake or no-op. **Where to draw that list is not a CLI
question**, and the whole of the reasoning — what `# leaf` means and where the
marker misses, which leaves are worth granting, which stand-in each interface
gets — is [conventions/component-harness](../conventions/component-harness.md#choosing-where-to-cut).
The rest of this section is what the command itself does.

The check reads every `.go` file in the module and reports the position of each
field it finds filled in, in either form:

```go
diogen.Replacements{SecurityAdapter: spy}   // the literal handed to Boot
cut.SecurityAdapter = spy                   // filled in field by field
```

Two things are excluded. The generated container file declares the struct and
assigns every field — but only that file, not its directory, so a component test
sitting beside `InitAppWith` is read like any other. And `tests/internal/` is the
harness: it hands in the real database and the connections a boot demands, which
is the platform owner's own decision, so measuring it against the platform
owner's own list would deny the thing the list exists to allow.

A grant is keyed by import path and interface, never by field name — the field
name is diogen's, and it changes when registrations do.

The report writes a package holding more than one interface one per line, so a
marker sits on the interface it describes — the common case, because a domain
declares the port of an external adapter next to its own repositories. A single
interface stays inline, where there is nothing to tell apart:

```yaml
replacements:
  allow:
    gitlab.com/gorib/sql: [Db]  # leaf

    example.com/platform/warehouse/domain/warehouse/service:
      - FacilityRepository
      - SecurityAdapter  # leaf, allowed
      - UnitRepository
```

`# leaf` on a line means the seam's default constructor takes nothing else out of
the container. It is read off the body of `InitAppWith`, so it is a fact about
the graph and not a recommendation — the marker misses the real process boundary
in both directions, and not every leaf is worth granting. Both cases, with what
to do instead, are in
[component-harness](../conventions/component-harness.md#what-the-marker-cannot-know).

**Mocks are not generated by waffle**, by this command or any other. A test that
needs one carries its own `//go:generate go tool mockgen` line — see
[component-harness](../conventions/component-harness.md#a-mock-is-a-declaration-a-spy-is-an-observation).

`make replacements` runs the check and prints its report. It is deliberately
not part of `make generate`: it reads the container, so it wants a tree that
already compiles. It is the fast way to ask — the same comparison rides in
`make test`, which boots the whole component suite to get there.

## Next

| | |
|---|---|
| The whole path a scaffolded service takes | [Getting Started](../getting-started.md) |
| Where the generated tree comes from and why it is shaped that way | [Recommended architectures](../architectures.md) |
| What each generated registration means | [di/overview](../di/overview.md) |
| What a component test may replace, and where to cut | [conventions/component-harness](../conventions/component-harness.md) |
