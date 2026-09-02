# Waffle Documentation

**Waffle gives a Go microservice one way to do each thing.**

A meta-framework that connects compile-time DI, input codegen, HTTP/gRPC, migrations and messaging, so a service written by one person reads like a service written by another.

That one way is about the **path**, not the tools on it. The entry point takes three things — a logger, a tracing provider and a list of commands. Everything else is a blank import you can leave out: bring your own query layer, your own transport, your own validator.

And leaving is as cheap as arriving. Delete a `//go:generate` line and the file it produced stays where it is, as ordinary Go you keep editing by hand — no generated container imports `gitlab.com/gorib/di` at all, and dropping whisk costs copying two small files. Waffle itself goes the same way: write your own `main`, build the container, keep whatever architecture and layout you like. See [What is optional](docs/opting-out.md) for what each tool replaces, what skipping it costs, and how to walk back out.

## Contents

### Getting Started
- [Introduction](docs/introduction.md) — What waffle is, ecosystem map, architecture overview
- [Getting Started](docs/getting-started.md) — Install the CLI, create a service, first run
- [What is optional](docs/opting-out.md) — What each tool replaces, the mandatory core, and the cost of skipping

### Core
- [Application](docs/core/application.md) — Lifecycle, `RunUntilStop`, graceful shutdown, signal handling
- [Commands](docs/core/commands.md) — `Command` interface, `CommandList`, writing custom commands
- [Context](docs/core/context.md) — CLI context, session metadata, correlation IDs

### CLI Scaffolding
- [Overview](docs/cli/overview.md) — Project root detection, template system, idempotent writes
- [init:service](docs/cli/init-service.md) — Bootstrap a new microservice project
- [make:domain](docs/cli/make-domain.md) — Model, service, postgres adapter, stored struct
- [make:domain:grpc](docs/cli/make-domain-grpc.md) — Proto definition, gRPC handler, error mapping
- [make:domain:migrate](docs/cli/make-domain-migrate.md) — Migration runner, SQL files, goose format
- [agents](docs/cli/agents.md) — Regenerate AGENTS.md (run after every waffle upgrade)

### Tools
- [HTTP](docs/tools/http.md) — chi-based server, middleware registry, route registration
- [HTTP Handler](docs/tools/http-handler.md) — Handler signature, request DTOs, responses
- [gRPC](docs/tools/grpc.md) — gRPC server, interceptors, error mapping
- [gRPC Handler](docs/tools/grpc-handler.md) — Handler struct, bare interfaces, error mapping
- [Migrate](docs/tools/migrate.md) — Database migrations, multiple sources, goose subcommands
- [Scribus](docs/tools/scribus.md) — RabbitMQ consumer, single and batch handler modes
- [Cron](docs/tools/cron.md) — Running other commands on a schedule
- [Hilton](docs/tools/hilton.md) — Tracing and metrics behind one interface

### Dependency Injection
- [Overview](docs/di/overview.md) — How `diogen` works, `InitXxx` convention, compile-time safety
- [Tags](docs/di/tags.md) — Every DI tag with registration and collection examples

### Conventions
- [Input Parsing](docs/conventions/input-parsing.md) — Binding and validation by codegen, custom tags, escape hatches
- [DTO Conversion](docs/conventions/dto-conversion.md) — goverter pattern, project-wide whitelist of conversion helpers

### Reference
- [Interfaces](docs/reference/interfaces.md) — Full interface listing for all packages
- [Running locally](docs/reference/running-locally.md) — What each module needs running and in `.env`
- [Environment Variables](docs/reference/environment.md) — All env vars read by waffle tools
