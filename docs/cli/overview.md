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
| `waffle make:domain:migrate <name>` | Add database migrations for an existing domain |

## How it works

### Project root detection

Every scaffolding command walks up the directory tree from the current working directory looking for a `go.mod` file. The module name in that file becomes the base import path used in all generated files. You can run `waffle` from any subdirectory of your project.

### Template rendering

Each command ships embedded Go templates. When you run a command, waffle renders each template with the parameters you provide (service name, domain name, timestamp) and writes the result to the target path.

**Idempotent writes:** If the target file already exists, waffle skips it without error. You can safely re-run commands; only missing files are created.

### Post-generation hooks

Some commands run additional steps after writing files:

- `init:service` — runs `go get gitlab.com/gorib/digen` then `make .tidy generate`
- `make:domain` — runs `go get gitlab.com/gorib/storage/sql/cmd/store` then `make .tidy generate`
- `make:domain:grpc` — runs `make .tidy generate`
- `make:domain:migrate` — runs `make .tidy generate`

The `make generate` step re-runs code generators (DI container, stored struct mappers) so the project compiles immediately after scaffolding.

### Template functions

Templates have access to two extra functions beyond the standard library:

| Function | Example |
|----------|---------|
| `plural` | `plural "user"` → `"users"` |
| `title` | `title "user_profile"` → `"User_profile"` |

## Detailed pages

- [init:service](init-service.md)
- [make:domain](make-domain.md)
- [make:domain:grpc](make-domain-grpc.md)
- [make:domain:migrate](make-domain-migrate.md)
