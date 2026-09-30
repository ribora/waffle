# Waffle documentation

**Waffle gives a Go microservice one way to do each thing.** That one way is
about the **path**, not the tools on it: the entry point takes a logger, a
tracing provider and a list of commands, and everything else is a blank import
you can leave out.

New here? [Introduction](introduction.md) is what waffle is,
[Getting Started](getting-started.md) is an empty directory answering `curl` in
about ten minutes, and [What is optional](opting-out.md) is what you are signing
up for.

## Reading paths

| If you are | Read, in order |
|---|---|
| starting a service | [Getting Started](getting-started.md) → [Recommended architectures](architectures.md) → [Running locally](reference/running-locally.md) |
| adding a transport to one that exists | [Commands](core/commands.md) → the tool page → the matching handler page |
| wiring something up | [DI overview](di/overview.md) → [Tags](di/tags.md) |
| setting a service's tests up | [Your first component test](first-component-test.md) → [Component harness](conventions/component-harness.md) → [tests:replacements](cli/overview.md#testsreplacements) |
| deciding how changes get made | [AI-native development](conventions/ai-native.md) → [cli/agents](cli/agents.md#what-is-in-the-file) |
| deciding whether to adopt waffle | [Introduction](introduction.md) → [What is optional](opting-out.md) → [Recommended architectures](architectures.md) |
| debugging a service that will not start | [Running locally](reference/running-locally.md#when-it-does-not-start) → [Environment Variables](reference/environment.md) |

## Getting Started

- [Introduction](introduction.md) — What waffle is, ecosystem map, architecture overview
- [Recommended architectures](architectures.md) — Single service, modular monolith, service fleet, and what it takes to change your mind
- [Getting Started](getting-started.md) — Install the CLI, scaffold a service, first route answering `curl`
- [Your first component test](first-component-test.md) — From `tests:harness` to one green scenario and one red one
- [What is optional](opting-out.md) — What each tool replaces, the mandatory core, and the cost of skipping

## Core

- [Application](core/application.md) — Lifecycle, `RunUntilStop`, graceful shutdown, signal handling
- [Commands](core/commands.md) — `Command` interface, the built-in set, writing your own
- [Context](core/context.md) — What a command's context carries, and why nothing takes `os.Args`

## CLI scaffolding

- [Overview](cli/overview.md) — Every command, naming rules, `waffle.yaml`, `tests:replacements`
- [init:service](cli/init-service.md) — Bootstrap a new microservice project
- [make:domain](cli/make-domain.md) — Model, service, postgres adapter, stored struct
- [make:domain:http](cli/make-domain-http.md) — Router, handler, request DTOs, convert package
- [make:domain:grpc](cli/make-domain-grpc.md) — Proto definition, gRPC handler, error mapping
- [make:domain:migrate](cli/make-domain-migrate.md) — Migration runner, SQL files, goose format
- [agents](cli/agents.md) — Regenerate AGENTS.md (run after every waffle upgrade)

## Tools

Each tool is a `Command` pulled in by one blank import in `config/di/modules.go`.

- [HTTP](tools/http.md) — chi-based server, middleware registry, OpenAPI generation
- [HTTP Handler](tools/http-handler.md) — Handler signature, request DTOs, responses
- [gRPC](tools/grpc.md) — gRPC server, interceptors, error mapping
- [gRPC Handler](tools/grpc-handler.md) — Handler struct, bare interfaces, error mapping
- [Migrate](tools/migrate.md) — Database migrations, multiple sources, goose subcommands
- [Scribus](tools/scribus.md) — Event consumer over a broker driver, single and batch modes
- [Outbox](tools/outbox.md) — Events that commit with the change that caused them
- [Cron](tools/cron.md) — Running other commands on a schedule
- [Hilton](tools/hilton.md) — Tracing and metrics behind one interface

## Dependency injection

- [Overview](di/overview.md) — How `diogen` works, the `InitXxx` convention, what generation refuses
- [Tags](di/tags.md) — Every DI tag with registration and collection examples

## Conventions

- [Input Parsing](conventions/input-parsing.md) — Binding and validation by codegen, custom tags, escape hatches
- [DTO Conversion](conventions/dto-conversion.md) — goverter pattern, project-wide whitelist of conversion helpers
- [AI-native development](conventions/ai-native.md) — Specification before code, the component-test cut, and what is not built yet
- [AI-native field notes](conventions/ai-native-field-notes.md) — The incidents behind Rule #11 and its modes, dated
- [Component harness](conventions/component-harness.md) — The resource seam, real postgres and elasticsearch, recorders instead of silent stand-ins
- [Flows](conventions/flows.md) — What happens after each way in, from the code and the suite, and what the two leave uncovered

## Reference

- [Interfaces](reference/interfaces.md) — Every interface a service implements or is handed
- [Running locally](reference/running-locally.md) — What each module needs running and in `.env`
- [Environment Variables](reference/environment.md) — All env vars read by waffle tools
