# CLI Overview

The `waffle` CLI is a standalone scaffolding tool. It generates the boilerplate for new services and domains so you can focus on business logic from the first commit.

## Install

```sh
go install gitlab.com/gorib/waffle/cmd/waffle@latest
```

## Commands

| Command | Description |
|---------|-------------|
| `waffle init:service` | Bootstrap a new microservice project |
| `waffle make:domain <name>` | Add a domain (model, service, postgres adapter) |
| `waffle make:domain:grpc <name>` | Add a gRPC handler for an existing domain |
| `waffle make:domain:http <name>` | Add an HTTP handler for an existing domain |
| `waffle make:domain:scribus [--batch] <name> <event> [routing-key]` | Add an AMQP consumer for an existing domain; `--batch` scaffolds a `BatchHandler` instead of a per-message `Handler` |
| `waffle make:domain:migrate <name>` | Add database migrations for an existing domain |
| `waffle make:domain:repository <type> <name> <entity>` | Add a secondary repository (`type` is currently only `sql`) |
| `waffle make:domain:migration <name> <description>` | Add a new SQL migration |
| `waffle make:domain:command <name> <command>` | Add a CLI subcommand scoped to a domain |
| `waffle make:converter <path>` | Scaffold a goverter convert.go at the given path |
| `waffle agents` | Regenerate `AGENTS.md` after a waffle upgrade ([details](agents.md)) |

`--help` on any of them prints its description and operands. A failing command
exits non-zero, so a `make` target that calls one stops.

### Names

A domain, entity, event or command name becomes a Go identifier — the domain
generator emits `var <name>ManagementInstance` and `Init<Name>Domain` — as well as
a directory and, for migrations, part of a SQL identifier. So it has to be a
lowercase Go identifier: a letter, then letters, digits or underscores, and not a
Go keyword. `user`, `order_item` and `user2` are fine; `user-profile`, `2fa`,
`User` and `type` are refused.

The check runs before anything is written, together with the check that
`config/di/modules.go` exists. A command that cannot finish leaves no files
behind — `waffle make:domain --help` used to scaffold a domain called `--help`
and stop halfway through.

Two operands are not identifiers and follow their own rules: the `<path>` of
`make:converter` has to stay inside the project, and the `<description>` of
`make:domain:migration` is one file-name segment, so it may hold dashes and dots
but not a separator.

### Routing keys

`make:domain:scribus` takes the routing key after the event, because the event
names the generated Go type and keys are usually dotted:

```sh
waffle make:domain:scribus user order_paid order.paid
```

The key defaults to the event, so `waffle make:domain:scribus user ping` binds
`ping` — which is why the event alone is enough when it happens to fit both jobs.
Give a dotted event with no key and the command says so rather than emitting a
type called `order.paidCommand`.

## How it works

### Project root detection

Every scaffolding command walks up the directory tree from the current working directory looking for a `go.mod` file. The module name in that file becomes the base import path used in all generated files. You can run `waffle` from any subdirectory of your project.

### Template rendering

Each command ships embedded Go templates. When you run a command, waffle renders each template with the parameters you provide (service name, domain name, timestamp) and writes the result to the target path.

**Idempotent writes:** If the target file already exists, waffle skips it without error. You can safely re-run commands; only missing files are created.

### Post-generation hooks

Some commands run additional steps after writing files:

- `init:service` — runs `go get -tool gitlab.com/gorib/di/cmd/diogen@latest` then `make .tidy generate`
- `make:domain` — runs `go get -tool gitlab.com/gorib/sql/cmd/store@latest` then `make .tidy generate`
- `make:domain:grpc` — runs `go generate ./api/...` (skipped if `proto_dir` is outside the module) then `make .tidy generate`
- `make:domain:migrate` — runs `make .tidy generate`

The `make generate` step re-runs code generators (DI container, stored struct mappers) so the project compiles immediately after scaffolding.

### Template functions

Templates have access to two extra functions beyond the standard library:

| Function | Example |
|----------|---------|
| `plural` | `plural "user"` → `"users"` |
| `title` | `title "user_profile"` → `"User_profile"` |

## waffle.yaml

Project-level conventions live in `waffle.yaml`, next to `go.mod`. `init:service` creates it with the defaults below; generators read it instead of hardcoding paths.

| Key | Default | Meaning |
|-----|---------|---------|
| `api_import` | `<module>/api/pb` | Import path prefix for generated proto stubs |
| `proto_dir` | `api/proto` | Where `.proto` sources are written |
| `mapper` | `"dto"` | `"dto"` scaffolds nothing extra (hand-written conversions); `"goverter"` scaffolds a goverter `convert/convert.go` |

`<service>` is the last path segment of the module name. A missing `waffle.yaml`, or one missing some keys, behaves exactly like a file containing only the defaults above — the two are indistinguishable to every command. An unknown key (e.g. a typo) is a hard error, and an invalid `mapper` value (anything other than `dto` or `goverter`) is a hard error.

The whisk generator keeps its own `whisk.yaml` beside it, on the same missing-is-defaults rule. `make:domain:http` and `make:domain:scribus` write it the first time they pull whisk in, and never overwrite one you have edited; the keys are documented in [whisk's README](https://gitlab.com/gorib/whisk#whiskyaml).

## Detailed pages

- [init:service](init-service.md)
- [make:domain](make-domain.md)
- [make:domain:grpc](make-domain-grpc.md)
- [make:domain:migrate](make-domain-migrate.md)
- [agents](agents.md)

The remaining commands — `make:domain:http`, `make:domain:scribus`,
`make:domain:repository`, `make:domain:migration`, `make:domain:command` and
`make:converter` — have no page of their own. They follow the same rules as the
ones above (project root detection, idempotent writes, `make generate`
afterwards); what they produce is described where the produced thing is
documented: [tools/http-handler](../tools/http-handler.md),
[tools/scribus](../tools/scribus.md), [tools/migrate](../tools/migrate.md),
[core/commands](../core/commands.md) and
[conventions/dto-conversion](../conventions/dto-conversion.md).
