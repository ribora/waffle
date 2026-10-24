# Introduction

[Docs](README.md)

Waffle is a meta-framework for building Go microservices: one way to do each thing, and one place to look for it. It replaces no individual tool — it connects specialized gorib packages into a predictable shape with almost no wiring code, and every tool on the path is something you opt into.

## The Golden Path

Every production microservice eventually solves the same problems: starting a server, running migrations, consuming a queue, wiring dependencies, handling errors, and shutting down gracefully. Each of those problems has a good solution in the gorib ecosystem. Waffle defines **the path through them** so you do not have to rediscover it on every project.

Following the Golden Path means:

- Your project structure is predictable across teams and services.
- New services are scaffolded in seconds, not hours.
- Dependency injection is compile-time, with no reflection at runtime.
- Every runtime concern — HTTP, gRPC, AMQP, migrations — is a pluggable `Command` the app discovers automatically.

## Ecosystem map

Waffle orchestrates these gorib packages:

| Package | Role |
|---------|------|
| `gitlab.com/gorib/di` | Compile-time dependency injection |
| `gitlab.com/gorib/pry` | Structured logging |
| `gitlab.com/gorib/failure` | Typed, structured errors |
| `gitlab.com/gorib/session` | Request/command context |
| `gitlab.com/gorib/env` | Type-safe environment variable reading |
| `gitlab.com/gorib/http` | chi-based HTTP server, typed handlers, OpenAPI generation |
| `gitlab.com/gorib/grpc` | gRPC server helpers |
| `gitlab.com/gorib/whisk` | Codegen for reading and validating input from any source — HTTP, AMQP, raw bytes (see [conventions/input-parsing](conventions/input-parsing.md)) |
| `gitlab.com/gorib/sql` | SQL query builder and repository |
| `gitlab.com/gorib/amqp` | RabbitMQ consumer and publisher (see [tools/scribus](tools/scribus.md)) |
| `gitlab.com/gorib/hilton` | Distributed tracing & metrics abstraction (see [tools/hilton](tools/hilton.md)) |
| `gitlab.com/gorib/redis` | Redis client with command tracing |
| `gitlab.com/gorib/elastic` | Elasticsearch client with tracing |

`redis` and `elastic` are registered the same way as `sql`, `pry` and `hilton` — a blank import of the matching `config/service/…` module. See [reference/environment](reference/environment.md) for the variables each of them reads.

These are not orchestrated by waffle, but a service on the Golden Path usually pulls them in directly:

| Package | Role |
|---------|------|
| `gitlab.com/gorib/criteria` | Filter expressions — the `and`/`or` trees `sql` turns into a `WHERE` clause |
| `gitlab.com/gorib/convert` | Conversion helpers for goverter `extend` directives (see [conventions/dto-conversion](conventions/dto-conversion.md)) |
| `gitlab.com/gorib/dispatcher` | In-process typed event dispatch, no broker |
| `gitlab.com/gorib/delay` | Saga helper — steps with compensations, rolled back in reverse |

## Architecture

A waffle service is structured around three layers:

```
cmd/app/main.go          ← entry point, wires DI, calls app.RunUntilStop()
config/di/               ← hand-written DI registrations
config/diogen/           ← generated DI container (do not edit)
domain/<name>/
  model/                 ← entities and domain errors
  service/               ← business logic
  adapter/postgres/      ← database adapters (stored structs + queries)
  cmd/grpc/<name>/       ← gRPC handler
  cmd/migrate/           ← migration runner
api/                     ← API initialization, proto definitions
```

### Commands

The app discovers **Commands** at startup. Each runtime concern is a command, and
waffle ships six: `http`, `grpc`, `scribus`, `outbox`, `cron` and `migrate`. You
pick which ones your service has by adding the matching blank import to
`config/di/modules.go` — there is no registry to edit and no flag to pass. The
full table, with the import line and the daemon marker for each, is
[core/commands](core/commands.md#built-in-commands).

Commands are invoked by name:

```sh
./app http
./app grpc
./app migrate up
./app scribus
```

A command your service needs and waffle does not have is a type implementing
`app.Command` and one `di.Tag(app.CommandTag)` — it lands in the same registry
and the same `--help`. See [core/commands](core/commands.md#writing-a-custom-command).

## Package layout

```
waffle/
├── app/           Core application lifecycle and command system
├── container/     Reads a service's diogen container from source: seams, env reads
├── boundaries/    The check behind the modular monolith's boundary rule
├── tools/
│   ├── http/      HTTP server command and middleware registry
│   ├── grpc/      gRPC server command, middleware and error mapping
│   ├── migrate/   Database migration command (goose)
│   ├── cron/      Scheduler that runs other commands
│   ├── outbox/    Transactional outbox: rows, relay and drivers
│   ├── tap/       Hook where the tools report messages and routes; silent unless a test listens
│   └── scribus/   Event consumer command, over a broker driver
├── config/        The modules a service pulls in by blank import
└── cmd/waffle/    Scaffolding CLI (waffle init:service, make:domain, …)
```

## Testing

A component test boots the real container over real infrastructure and replaces
only the leaves. The pieces it stands on live next to what they describe —
`sql/sqltest`, `redis/redistest`, `elastic/elastictest`, `pry.Recorder()`,
`scribus.Recorder()`, `tools/grpc.Serve`, `tools/http.Serve` — and the harness that
puts them together from the container is specrig's: see
[the waffle adapter of specrig-go](https://github.com/specrig/specrig-go/tree/main/waffle).
