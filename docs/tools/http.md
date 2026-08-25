# HTTP Tool

The HTTP tool wraps `gitlab.com/gorib/http/server` into a waffle `Command`. Blank-import its config package to register the `http` command.

## Setup

Add a single blank import to `config/di/modules.go`:

```go
// config/di/modules.go
package di

import (
    _ "gitlab.com/gorib/waffle/config/tools/http"
    // ... other modules ...
)
```

This pulls in the registrations from [`config/tools/http/init.go`](../../config/tools/http/init.go): the `http` command, the listen address, the server mux, and the standard middleware chain (compress, health, session, tracing, log, error, recover). Use the manual form — copy those registrations into your `InitDi()` — only when you need to override one.

Run `make generate`, then start the server:

```sh
./app http
./app http --listen :9000
```

## Listen address

The address is wired automatically from `DAEMON_HOST` / `DAEMON_PORT` (see `config/tools/http`). Override it at runtime with the `--listen` / `-l` flag, which takes precedence:

```sh
./app http --listen 0.0.0.0:8080
```

## Registering routes

Implement `server.Router` — a single `Setup(mux server.Mux)` method. The scaffolded router under `domain/<x>/cmd/http` is the conventional place: it injects the standard middleware chain, applies it, then registers routes.

```go
package http

import (
    "gitlab.com/gorib/http/server"
    "gitlab.com/gorib/http/server/middleware"
)

type router struct {
    handler  *userHandler
    standard *middleware.Standard
}

func (r *router) Setup(mux server.Mux) {
    mux.Use(r.standard.All()...) // global middleware chain

    mux.Get("/v1/users/{id}", server.Handle(r.handler.Get))
    mux.Post("/v1/users", server.Handle(r.handler.Create))
}
```

Route patterns use chi syntax — `{id}`, `{id:[0-9]+}`, trailing `/*` wildcard. Register the router in DI:

```go
di.Wire[server.Router](http.NewRouter)
```

## OpenAPI

The `--openapi` flag derives an OpenAPI 3.1 document from the registered routes, writes it to the given path and exits without serving:

```sh
./app http --openapi api/openapi.yaml
```

The flag runs the real wiring — the router and everything it depends on is constructed — so the spec describes the same routes the server would serve.

Only routes registered with `server.Bind` reach the spec: `Bind` captures the request and response types, `server.Handle` has nothing to capture. A `Handle` route is invisible to the generator; if you documented one by hand it stays in the file and is reported as doc-only.

```go
func (r *router) Setup(mux server.Mux) {
    mux.Use(r.standard.All()...)

    mux.Get("/v1/users/{id}", server.Bind(r.handler.Get))
    mux.Post("/v1/users", server.Bind(r.handler.Create))
}
```

A `Bind` handler takes the parsed request and returns a typed response:

```go
func (h *userHandler) Get(ctx context.Context, req *getRequest) (*server.Response[userResponse], error)
```

### Code owns structure, you own prose

The first run writes a fresh file. Every later run merges into the existing one:

| | |
|---|---|
| **Added** when the code has it and the file doesn't | schemas, fields, parameters, operations — each marked `TODO(openapi)`, so unfinished prose is greppable |
| **Kept in sync** on every run | `required`, nullability, `operationId` |
| **Left alone** | `summary`, `description`, examples, extra responses, `format` on existing fields, and everything outside `paths` and `components` |
| **Warned** | anything in the file the code doesn't produce — a hand-written operation, or something stale after a rename |
| **Error** | a field or parameter whose type contradicts the code, or a field the code no longer has |

Warnings are logged and the command still succeeds. Structural conflicts make it exit non-zero; the merged file is written first, so you can open it and see what disagreed.

### What the DTO contributes

The request type registered with `Bind` supplies both the parameters and the body schema; the response type supplies the 200 schema.

| In the DTO | In the spec |
|---|---|
| `scan:"path=id"`, `scan:"query=limit"`, `scan:"header=X-Request-Id"` | a parameter in that location; `path` is always required |
| `scan:"embed"` | the embedded struct's fields are inlined |
| `json:"email"`, or a protobuf `json=` name | a body property; a field with neither tag is skipped |
| `validate:"required"` | listed in `required` |
| `validate:"email"`, `uuid`, `url`, `hostname`, `datetime` | `format` |
| `validate:"oneof=new used"` | `enum` |
| `validate:"min=2,max=64"` | `minLength`/`maxLength`, `minimum`/`maximum` or `minItems`/`maxItems`, by type |
| any other `validate` rule | appended to the description as `Constraints: …` |
| a pointer field that isn't required | nullable — `type: [string, "null"]`, or `anyOf` for a `$ref` |
| a type with a `Values() []string` method | `enum` |
| `time.Time`, `uuid.UUID`, protobuf well-known types | their standard formats |

Parameters that share a name across requests share one component when their location, type and requiredness agree. When they disagree, the generator splits them into `pkg.ReqType.name` components and warns, rather than merging two different contracts under one name.

`operationId` is derived from the handler — its method name, extended with the package, the path and the HTTP method only as far as uniqueness requires. Rename a handler and the id follows on the next run.

### In CI

```sh
./app http --openapi api/openapi.yaml
git diff --exit-code api/openapi.yaml
```

The first command fails on a structural conflict, the second on drift — code changed and nobody regenerated. The app boots to build the router, so give the step whatever config the service needs, even though it never listens. Expect one noisy commit when adopting this on a hand-written file: the merge round-trips the YAML.

## Middleware

Middleware is a `server.Layer`: `func(next server.Handler) server.Handler`. The framework ships a standard chain via `*middleware.Standard` — compress, health, session, tracing, log, error, recover — applied with `mux.Use(r.standard.All()...)`. Each layer in it is a replaceable default (`di.Fallback`): register your own, e.g. `di.Wire[middleware.Session](NewMySession)`, and it wins over the default.

### Custom middleware

Write a `server.Layer` and add it to the chain:

```go
func NewAuthLayer(publicKey *ecdsa.PublicKey) server.Layer {
    return func(next server.Handler) server.Handler {
        return func(src server.Source) (server.Renderer, error) {
            // validate the JWT from src.Header("Authorization")
            return next(src)
        }
    }
}
```

Apply it globally in `Setup` (place cross-cutting concerns like auth/cors outermost — layers run outer→inner in registration order):

```go
func (r *router) Setup(mux server.Mux) {
    mux.Use(r.auth)              // outermost
    mux.Use(r.standard.All()...)
    // routes ...
}
```

or scope it to a route / group:

```go
mux.Get("/admin/{id}", r.auth, server.Handle(r.handler.GetAdmin))

admin := mux.Group("/admin", r.auth)
admin.Get("/{id}", server.Handle(r.handler.GetAdmin))
```

## Graceful shutdown

The HTTP server registers itself with the app closer. On `SIGINT` / `SIGTERM` the server's graceful shutdown is triggered and in-flight requests are allowed to complete, bounded by the configured shutdown timeout.
