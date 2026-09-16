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
| `waffle make:domain:entity <name> <entity>` | Add an entity to an existing domain — model, repository interface, adapter, table and DI, the same set the domain's own entity gets. `--storage` picks the adapter and defaults to `sql`, the only one there is; an entity always gets a repository |
| `waffle make:domain:migration <name> <description>` | Add a new SQL migration |
| `waffle make:domain:command <name> <command>` | Add a CLI subcommand scoped to a domain |
| `waffle make:converter <path>` | Scaffold a goverter convert.go at the given path |
| `waffle tests:harness [init]` | Report the component harness, and write what is missing with `init` ([details](../conventions/component-harness.md)) |
| `waffle tests:replacements [check] [--container=<path>]` | List everything the containers expose and what `replacements.allow` grants; `check` fails if a test replaces a field the list does not name |
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
| `replacements.allow` | empty | Interfaces a test may supply on the container's `Replacements`, keyed by import path: `gitlab.com/gorib/sql: [Db]`. A permission list — an empty one allows nothing |

`<service>` is the last path segment of the module name. A missing `waffle.yaml`, or one missing some keys, behaves exactly like a file containing only the defaults above — the two are indistinguishable to every command. An unknown key (e.g. a typo) is a hard error, and an invalid `mapper` value (anything other than `dto` or `goverter`) is a hard error.

The whisk generator keeps its own `whisk.yaml` beside it, on the same missing-is-defaults rule. `make:domain:http` and `make:domain:scribus` write it the first time they pull whisk in, and never overwrite one you have edited; the keys are documented in [whisk's README](https://gitlab.com/gorib/whisk#whiskyaml).

## Detailed pages

- [init:service](init-service.md)
- [make:domain](make-domain.md)
- [make:domain:grpc](make-domain-grpc.md)
- [make:domain:migrate](make-domain-migrate.md)
- [agents](agents.md)

The remaining commands — `make:domain:http`, `make:domain:scribus`,
`make:domain:entity`, `make:domain:migration`, `make:domain:command`,
`make:converter`, `tests:harness` and `tests:replacements` — have no
page of their own. They
follow the same rules as the ones above (project root detection, idempotent
writes, `make generate` afterwards); what they produce is described where the
produced thing is documented: [tools/http-handler](../tools/http-handler.md),
[tools/scribus](../tools/scribus.md), [tools/migrate](../tools/migrate.md),
[core/commands](../core/commands.md),
[conventions/dto-conversion](../conventions/dto-conversion.md) and
[di/overview](../di/overview.md#testing-with-the-container).

`tests:harness` takes one optional operand: a bare run reports which of the
harness files are there, and `init` writes the rest, leaving an existing file
alone. Which files those are, and why they sit where they do, is
[conventions/component-harness](../conventions/component-harness.md#scaffolding-it).

Four things come from the container rather than from a flag, because a flag would
close a set the container already knows:

- **the systems.** A seam of type `gitlab.com/gorib/sql.Db` means postgres, and
  the harness gets `sqltest` plus every `domain/*/cmd/migrate` it finds; a
  `gitlab.com/gorib/redis.Db` seam means `redistest`. Without either — an API
  gateway, a proxy — the set is `resource.New()`, whose `Up`, `Reset` and `Down`
  do nothing. Adding a resource later is editing that literal.
- **the transports.** `NewGrpcCommand` in the container gives an in-process gRPC
  server, `NewHttpCommand` gives `waffleHttp.Serve` over the container's own mux
  and router, and a service that has both gets both behind one `Clients` — two
  of the nine in sherpa do.
- **the handlers and the interceptor order**, copied out of the daemon's own
  construction call, so the server a scenario talks to is chained like the
  daemon's.
- **the dead connections.** Every `grpc.ClientConnInterface` seam becomes
  `grpcClient.Dead(...)`, because replacing a typed client does not stop the
  connection under it from being built.

What it cannot know — a second resource, extra environment, which typed client
belongs to which handler — it leaves as a named gap rather than a guess.

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
mock, spy, hand-written fake or no-op. Which form to use is the test author's
choice, because it follows from the assertion; where the boundary lies is not.

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
name is diogen's, and it changes when registrations do. One consequence worth
knowing: an interface serving two fields is granted for both. `google.golang.org/
grpc: [ClientConnInterface]` grants every connection in the container at once,
which is a reason not to grant it.

A **composite** field — `interface{ A; B }`, produced when registrations share a
container field — is granted only when every constituent is, because replacing
the field replaces every contract it merges. Composites are the most informative
lines in the report rather than noise: a composite names the contracts one
management object owes, and each constituent is a caller, which is a place a
scenario binds.

An entry marked `# leaf` has a default constructor that takes nothing else out of
the container, which makes it a boundary of the process rather than a node inside
it. It is read off the body of `InitAppWith`, so it is a fact about the graph and
not a recommendation — but it is the fact that separates the two kinds of test:

- a **component test** replaces leaves and lets everything else run for real.
  Replacing an interior seam instead deletes the subtree below it, which is the
  code the test was meant to exercise.
- a **unit test** builds the object under test itself, so `Replacements` plays no
  part, and this list has nothing to say about it.

A package holding more than one interface is written one per line, so the marker
sits on the interface it describes — which is the common case, because a domain
declares the port of an external adapter next to its own repositories. A single
interface stays inline, where there is nothing to tell apart:

```yaml
replacements:
  allow:
    gitlab.com/gorib/sql: [Db]  # leaf

    gitlab.com/sherpa-erp/sherpa/warehouse/domain/warehouse/service:
      - FacilityRepository
      - SecurityAdapter  # leaf, allowed
      - UnitRepository
```

The marker is structural, and the boundary of the container is not always the
boundary of the process — it misses in both directions:

- reaching another service over gRPC, the leaf is `grpc.ClientConnInterface`,
  while the seam worth granting is the generated `*ServiceClient` one hop above
  it. Faking the connection would mean answering with encoded protobuf.
- behind a factory that builds its clients from runtime data, the real boundary
  is below the leaf and outside the container entirely. It cannot be reached
  through `Replacements` at all — that one is for a unit test of the factory, and
  the component test still cuts at the factory's own port.

A leaf is also not always worth granting. `sql.Db` is one, but a component test
wants a real connection to a test database, and a stand-in for a dependency
carrying a language the code itself generates makes the test check the language
rather than the behaviour. Same for a leaf that only has to be non-nil for the
boot — the logger, a consumer, the connection under a replaced client: the
harness supplies it, so no grant is needed.
[conventions/component-harness](../conventions/component-harness.md) has the
reasoning and the stand-ins that ship with each interface.

None of this is derivable, which is why the list is written by hand.

**Mocks are not generated by waffle.** A test that needs one carries its own
`//go:generate go tool mockgen` line, which keeps the mock beside its only user
and inside `go generate ./...`:

```go
//go:generate go tool mockgen -typed -package security -self_package <this package> -destination client_mock_test.go <this package> Client
```

Generating into a `_test.go` destination in the package under test is what makes
a private field reachable, and it keeps a single-use mock out of the production
build and out of everyone else's import graph.

`make replacements` runs the check. It is deliberately not part of `make
generate`: it reads the container, so it wants a tree that already compiles.
