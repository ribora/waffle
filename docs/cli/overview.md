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
| `waffle make:tests:component` | Scaffold a component test that boots the container through `InitAppWith` ([details](../di/overview.md#testing-with-the-container)) |
| `waffle make:tests:mocks [--suggest \| --write \| --check]` | Generate the mocks listed in `waffle.yaml`; the flags list, record and enforce what the container exposes |
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
| `mapper` | `"goverter"` | `"goverter"` scaffolds a goverter `convert/convert.go`; `"dto"` scaffolds nothing extra (hand-written conversions) |
| `mocks.generate` | empty | Interfaces to generate mocks for, keyed by import path: `gitlab.com/gorib/sql: [Db]` |
| `mocks.ignore` | empty | Seams deliberately left without a mock, same shape; `--check` passes once every seam is in one of the two |
| `mocks.dir` | `internal/dimock` | Where the generated mocks go, one flat package |

`<service>` is the last path segment of the module name. A missing `waffle.yaml`, or one missing some keys, behaves exactly like a file containing only the defaults above — the two are indistinguishable to every command. An unknown key (e.g. a typo) is a hard error, and an invalid `mapper` value (anything other than `dto` or `goverter`) is a hard error.

The whisk generator keeps its own `whisk.yaml` beside it, on the same missing-is-defaults rule. `make:domain:http` and `make:domain:scribus` write it the first time they pull whisk in, and never overwrite one you have edited; the keys are documented in [whisk's README](https://gitlab.com/gorib/whisk#whiskyaml).

## Detailed pages

- [init:service](init-service.md)
- [make:domain](make-domain.md)
- [make:domain:grpc](make-domain-grpc.md)
- [make:domain:migrate](make-domain-migrate.md)
- [agents](agents.md)

The remaining commands — `make:domain:http`, `make:domain:scribus`,
`make:domain:repository`, `make:domain:migration`, `make:domain:command`,
`make:converter`, `make:tests:component` and `make:tests:mocks` — have no page of
their own. They
follow the same rules as the ones above (project root detection, idempotent
writes, `make generate` afterwards); what they produce is described where the
produced thing is documented: [tools/http-handler](../tools/http-handler.md),
[tools/scribus](../tools/scribus.md), [tools/migrate](../tools/migrate.md),
[core/commands](../core/commands.md),
[conventions/dto-conversion](../conventions/dto-conversion.md) and
[di/overview](../di/overview.md#testing-with-the-container).

`make:tests:component` takes no operands: it finds the container by reading
`-output` off the `//go:generate` line that invokes diogen, so a service with two
containers gets a test beside each. The scaffolded test starts with a `t.Skip`,
because the container reads the environment while it wires and nothing can guess
which variables this test needs.

### make:tests:mocks

The seams a test can replace are the fields of the generated `Replacements`
struct, so that struct is what this command reads — parsed, not type-checked,
which is why it also works on a service whose dependencies do not currently
compile. The unit of the report is the interface, not the seam: several fields of
one type collapse to a single entry, and an instantiated generic collapses to the
generic interface mockgen actually mocks.

| | |
|---|---|
| no flags | run mockgen over `mocks.generate`, one invocation per import path |
| `--suggest` | print, as a paste-ready block, every interface listed in neither `mocks.generate` nor `mocks.ignore` |
| `--write` | add those to `mocks.generate`, leaving the rest of the file untouched |
| `--check` | exit non-zero if there are any, so CI notices a new dependency nobody classified |
| `--container=<path>` | read this generated file instead of discovering it; repeatable |

Composite seams — `interface{ A; B }`, produced when registrations share a
container field — are reported separately and never suggested: no single mock
satisfies them. Mock one constituent for a handler unit test instead.

An entry marked `# leaf` has a default constructor that takes nothing else out of
the container, which makes it a boundary of the process rather than a node inside
it. It is read off the body of `InitAppWith`, so it is a fact about the graph and
not a recommendation — but it is the fact that separates the two kinds of test:

- a **component test** replaces leaves and lets everything else run for real.
  Replacing an interior seam instead deletes the subtree below it, which is the
  code the test was meant to exercise.
- a **unit test** builds the object under test itself, so `Replacements` plays no
  part; the interior entries are in the config for their mocks alone.

A package holding more than one interface is written one per line, so the marker
sits on the interface it describes — which is the common case, because a domain
declares the port of an external adapter next to its own repositories. A single
interface stays inline, where there is nothing to tell apart:

```yaml
mocks:
  generate:
    gitlab.com/gorib/sql: [Db]  # leaf

    gitlab.com/sherpa-erp/sherpa/warehouse/domain/warehouse/service:
      - FacilityRepository
      - SecurityAdapter  # leaf
      - UnitRepository
```

The marker is structural, not advice, and the boundary of the container is not
always the boundary of the process — it misses in both directions:

- reaching another service over gRPC, the leaf is `grpc.ClientConnInterface`,
  while the seam worth replacing is the generated `*ServiceClient` one hop above
  it. Mocking the connection would mean answering with encoded protobuf.
- behind a factory that builds its clients from runtime data, the real boundary
  is below the leaf and outside the container entirely. A mock of it cannot reach
  the app through `Replacements` at all — it is for a unit test of the factory,
  and the component test still cuts at the factory's own port.

A leaf is also not always worth a mock. `sql.Db` is one, but a component test
hands it a real connection to a test database, so it belongs in `mocks.ignore`
while still being part of the cut. Same for a port whose production
implementation already has a null variant selected by data: nothing needs
replacing, so it is classified and left alone.

None of this is derivable, which is why both lists are curated by hand and
`mocks.generate` accepts any interface — seam or not.

Mocks land in one flat package, so two interfaces of the same name from different
packages would collide. Only those are renamed, by the shortest path suffix that
tells them apart: `MockVendorsServiceImageClient` next to
`MockProductServiceImageClient`, while everything unique keeps mockgen's own
`MockPaymentServiceClient`.

`make mocks` runs the generation. It is deliberately not part of `make generate`:
mockgen loads the packages the container names, so it wants a tree that already
compiles, and the mock set changes far less often than the container does.
