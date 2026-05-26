# Introduction

Waffle is an opinionated meta-framework for building Go microservices. It does not replace any individual tool — instead it connects specialized gorib packages into a coherent, predictable developer experience with almost no wiring code.

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
| `gitlab.com/gorib/http` | Fiber-based HTTP router |
| `gitlab.com/gorib/grpc` | gRPC server helpers |
| `gitlab.com/gorib/sql` | SQL query builder and repository |
| `gitlab.com/gorib/amqp` | RabbitMQ consumer |
| `gitlab.com/gorib/try` | Panic recovery |
| `gitlab.com/gorib/hilton` | Distributed tracing & metrics abstraction (see [tools/hilton](tools/hilton.md)) |

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
api/<service>/           ← API initialization, proto definitions
```

### Commands

The app discovers **Commands** at startup. Each runtime concern is a command:

| Built-in command | Enable with (blank import in `config/di/modules.go`)                  |
|-----------------|-----------------------------------------------------------------------|
| `http` | `_ "gitlab.com/gorib/waffle/config/tools/http"`                       |
| `grpc` | `_ "gitlab.com/gorib/waffle/config/tools/grpc"`                       |
| `migrate` | `_ "gitlab.com/gorib/waffle/config/tools/migrate"`                    |
| `scribus` (single mode) | `_ "gitlab.com/gorib/waffle/config/tools/scribus/single"`             |
| `scribus` (batch mode) | `_ "gitlab.com/gorib/waffle/config/tools/scribus/batch"`              |

You pick which commands your service needs by adding the corresponding blank import to `config/di/modules.go`. Commands are invoked by name:

```sh
./app http
./app grpc
./app migrate up
./app scribus amqp://... exchange queue:routing.key
```

## Package layout

```
waffle/
├── app/          Core application lifecycle and command system
├── tools/
│   ├── http/     HTTP server command and middleware registry
│   ├── grpc/     gRPC server command, middleware and error mapping
│   ├── migrate/  Database migration command (goose)
│   └── scribus/  RabbitMQ consumer command
└── cmd/waffle/   Scaffolding CLI (waffle init:service, make:domain, …)
```
