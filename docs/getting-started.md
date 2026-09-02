# Getting Started

## Prerequisites

- Go 1.27 or later
- `make`
- Access to the gorib module proxy / repository

## Install the CLI

```sh
go install gitlab.com/gorib/waffle/cmd/waffle@latest
```

Verify the installation:

```sh
waffle --help
```

## Create a new service

Run `init:service` inside a new empty directory:

```sh
mkdir my-service && cd my-service
waffle init:service
```

You will be prompted for the service name. The command then:

1. Scaffolds the full project layout
2. Runs `go get gitlab.com/gorib/di/cmd/diogen` to install the DI code generator
3. Runs `make .tidy generate` to tidy dependencies and generate the DI container

### Generated project layout

```
my-service/
├── api/
│   └── <service>/
│       ├── init.go          Service API initialization
│       └── proto/           Proto definition files (gRPC)
├── cmd/
│   └── app/
│       └── main.go          Entry point
├── config/
│   ├── config.go            Thin wrapper: func Init() (app.App, error)
│   └── di/
│       ├── di.go            Hand-written DI wiring (InitDi + //go:generate)
│       └── diogen/
│           ├── di.go        Hand-written diogen adapter (InitDi → InitApp)
│           └── di_gen.go    Generated DI container (do not edit)
├── .env                     Local environment variables
├── .gitignore
└── Makefile
```

## Run the app

```sh
go run ./cmd/app --help
```

The app prints available commands. Initially none are registered — add tools in the next step.

## Add your first tool

Open `config/di/modules.go` and add a blank import for the HTTP tool:

```go
// config/di/modules.go
package di

import (
    _ "gitlab.com/gorib/waffle/config/app"
    _ "gitlab.com/gorib/waffle/config/tools/http"
)
```

`config/di/di.go` stays empty for now — service-specific `di.Wire` / `di.Define` calls go there later.

See [tools/http.md](tools/http.md) for what the HTTP module expands to and how to override individual registrations.

Every module expects something in `.env`, and some expect infrastructure to be running — the HTTP tool needs `DAEMON_HOST` and `DAEMON_PORT`. See [Running locally](reference/running-locally.md) for the full list per module.

Run `make generate` to regenerate the DI container, then start the HTTP server:

```sh
go run ./cmd/app http
```

## Add a domain

Generate the domain scaffolding:

```sh
waffle make:domain user
```

This creates the model, service, postgres adapter, and DI configuration for a `user` domain. See [make:domain](cli/make-domain.md) for details.

## Read and validate input

Anything a service accepts — an HTTP body, a queue message, a file — is parsed by generated code rather than by hand. Put the tags on the struct, add one directive, run `make generate`:

```go
//go:generate go tool whisk $GOFILE

type createUser struct {
    Email string `json:"email" validate:"required,email"`
    Code  string `json:"code"  validate:"required,passcode"`
}
```

`passcode` is not a built-in — it is a tag added by writing one function in `pkg/validate`:

```go
// pkg/validate/rules.go
package validate

func IsPasscode(value string, args ...string) bool {
    return len(value) == 6
}
```

No registration, no flag, no fork: the func name is the tag, in any file of that directory. The same convention covers everything the generator can't express on its own — a `validate()` method for cross-field rules, a `parse()` method for fields it can't convert, and a custom decoder for body formats beyond JSON and XML. A custom tag that shadows a built-in one wins, so even the defaults are replaceable.

See [conventions/input-parsing.md](conventions/input-parsing.md).

## Upgrading waffle

When you bump waffle to a newer version, refresh the AI-agent instructions:

```sh
go get gitlab.com/gorib/waffle@latest
go get -tool gitlab.com/gorib/waffle/cmd/waffle@latest
waffle agents                  # refresh AGENTS.md with new conventions
go mod tidy
make generate
```

`waffle agents` rewrites `AGENTS.md` (the AI-agent instructions file that
ships with every waffle service) so AI coding assistants follow the current
framework conventions. Without this step, agents may keep using deprecated
patterns from the previous waffle version. See [agents](cli/agents.md).

## Next steps

| Topic | Page |
|-------|------|
| Application lifecycle | [core/application.md](core/application.md) |
| Writing commands | [core/commands.md](core/commands.md) |
| HTTP server setup | [tools/http.md](tools/http.md) |
| HTTP handler pattern | [tools/http-handler.md](tools/http-handler.md) |
| Input parsing and validation | [conventions/input-parsing.md](conventions/input-parsing.md) |
| gRPC server setup | [tools/grpc.md](tools/grpc.md) |
| gRPC handler pattern | [tools/grpc-handler.md](tools/grpc-handler.md) |
| Database migrations | [tools/migrate.md](tools/migrate.md) |
| RabbitMQ consumer | [tools/scribus.md](tools/scribus.md) |
| Dependency injection | [di/overview.md](di/overview.md) |
