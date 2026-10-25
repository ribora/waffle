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
| `waffle agents` | Regenerate `waffle.md` after a waffle upgrade, and bring it into the service's `AGENTS.md` ([details](agents.md)) |

`--help` on any of them prints its description and operands. A flag that takes a
value takes it either way, `--driver amqp` or `--driver=amqp`. A failing command
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

The check runs before anything is written, and so do the ones below, each of
which used to fail at the end of a command with its files already on disk. A
command that cannot finish leaves no files behind — `waffle make:domain --help`
used to scaffold a domain called `--help` and stop halfway through.

| Refused when | By |
|---|---|
| `config/di/modules.go` is missing, or has no line — commented out or active — for a module the command enables; the refusal prints the lines to add | every command that enables a module |
| `domain/<domain>` does not exist | `make:domain:http`, `make:domain:grpc`, `make:domain:scribus`, `make:domain:command`, `make:domain:entity`, `make:domain:migrate`, `make:domain:migration` |
| `domain/<domain>/cmd/migrate/main.go` does not exist, so nothing would apply the file | `make:domain:migration` |
| the other mode, another driver or, for `--batch`, a second `BatchHandler` is already wired | `make:domain:scribus` — see [tools/scribus](../tools/scribus.md#setup) |

Enabling a module whose line is in neither form is an error rather than a step
skipped in silence: the module would stay off, and nothing would say so until the
container or the binary failed.

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

Every scaffolding command walks up the directory tree from the current working directory looking for a `go.mod` file. The module name in that file becomes the base import path used in all generated files. You can run `waffle` from any subdirectory of your project: every path a command writes, reads or reports is relative to that root, and the tools it runs — `make`, `go generate`, `go get` — run there too. `waffle make:domain user` from `domain/order/` writes `domain/user/` beside it, not below it. The `<path>` of `make:converter` is relative to the root for the same reason.

### Template rendering

Each command ships embedded Go templates. When you run a command, waffle renders each template with the parameters you provide (service name, domain name, timestamp) and writes the result to the target path.

**Idempotent writes:** If the target file already exists, waffle skips it without error. You can safely re-run commands; only missing files are created.

### Post-generation hooks

Some commands run additional steps after writing files:

- `init:service` — installs the `diogen` and `waffle` tools, then runs `make .tidy generate`
- `make:domain` — installs the `store` tool and enables `config/service/sql` and `config/tools/migrate` (unless `--no-repository`), then runs `make .tidy` and `go generate ./domain/<domain>/...` — not the container, which refuses the domain until a transport reads it
- `make:domain:entity` — installs the `store` tool and enables `config/service/sql` and `config/tools/migrate`, then runs `make .tidy` and `go generate ./domain/<domain>/...` — not the container, which refuses the new repository until something reads it; the command prints the parameter to add to the management, and `make generate` after that
- `make:domain:grpc` — enables `config/tools/grpc`, runs `go generate ./api/...`, then `make .tidy generate`; when `proto_dir` is outside the module both runs are skipped and the command prints the steps to do by hand
- `make:domain:http` — enables `config/tools/http`, installs `whisk` (and `goverter`, under the default mapper) and writes `whisk.yaml`, then runs `make .tidy generate`
- `make:domain:scribus` — enables `config/tools/scribus/<driver>` and the mode module, installs `whisk`, then runs `make .tidy generate`
- `make:domain:command` — runs `make .tidy generate`
- `make:domain:migrate` — enables `config/service/sql` and `config/tools/migrate`, and stops there
- `make:outbox` — enables `config/service/sql`, `config/tools/migrate`, `config/tools/outbox` and the driver named by `--driver`, then runs `make .tidy generate`

`make:domain:migrate`, `make:domain:migration`, `make:converter` and `agents`
run no `make`; run `make generate` yourself afterwards.

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

`<service>` is the last path segment of the module name. A missing `waffle.yaml`, an empty one, one holding only comments, or one missing some keys, behaves exactly like a file containing only the defaults above — they are indistinguishable to every command. An unknown key (e.g. a typo) or an invalid `mapper` value (anything other than `dto` or `goverter`) is a hard error in the commands that read the file — the transports, `agents` and the two commands that regenerate `waffle.md` (`init:service`, `make:domain`); the others never open it.

What a component test may replace, the areas of Rule #11 and what a record cannot learn from the code are not waffle's keys: specrig reads them from `specrig.yaml`, beside it. See [the waffle adapter of specrig-go](https://github.com/specrig/specrig-go/tree/main/waffle).

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

The component harness, the list of what a test may replace and the records a
suite writes are specrig's, for a waffle service through
[the waffle adapter of specrig-go](https://github.com/specrig/specrig-go/tree/main/waffle): `go tool specrig init`
once after `init:service`, then `harness`, `replacements` and `surface`. A
project's flows are assembled from every service's records by specrig-go's
`specrig-flows`.

## Next

| | |
|---|---|
| The whole path a scaffolded service takes | [Getting Started](../getting-started.md) |
| Where the generated tree comes from and why it is shaped that way | [Recommended architectures](../architectures.md) |
| What each generated registration means | [di/overview](../di/overview.md) |
| What a component test may replace, and where to cut | [Component harness](https://github.com/specrig/specrig-go/blob/main/waffle/docs/harness.md) |
