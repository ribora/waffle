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
