# Waffle

**Waffle gives a Go microservice one way to do each thing.**

A meta-framework that connects compile-time DI, input codegen, HTTP/gRPC, migrations and messaging, so a service written by one person reads like a service written by another.

That one way is about the **path**, not the tools on it. The entry point takes three things — a logger, a tracing provider and a list of commands. Everything else is a blank import you can leave out: bring your own query layer, your own transport, your own validator.

And leaving is as cheap as arriving. Delete a `//go:generate` line and the file it produced stays where it is, as ordinary Go you keep editing by hand — no generated container imports `gitlab.com/gorib/di` at all, and dropping whisk costs copying a few small files. Waffle itself goes the same way: write your own `main`, build the container, keep whatever architecture and layout you like. See [What is optional](docs/opting-out.md) for what each tool replaces, what skipping it costs, and how to walk back out.

## Install

```sh
go install gitlab.com/gorib/waffle/cmd/waffle@latest
```

```sh
mkdir catalog
cd catalog
go mod init catalog
waffle init:service
```

## Documentation

**[docs/](docs/README.md)** is the full index, with a reading path for each way in. The four that answer most first questions:

| | |
|---|---|
| [Introduction](docs/introduction.md) | what waffle is, and which gorib packages it connects |
| [Getting Started](docs/getting-started.md) | an empty directory to a route answering `curl` |
| [Recommended architectures](docs/architectures.md) | single service, modular monolith, fleet — and the exit from each |
| [What is optional](docs/opting-out.md) | what every tool replaces, and the cost of skipping it |

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

## License

See [LICENSE](LICENSE).
