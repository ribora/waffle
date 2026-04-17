# HTTP Tool

The HTTP tool wraps `gitlab.com/gorib/http` (Fiber) into a waffle `Command`. Call `InitHttp()` in your DI configuration to register the `http` command.

## Setup

Copy the `InitHttp` registrations into your DI config:

```go
// config/di/di_http.go
package di

import (
    "gitlab.com/gorib/di"
    "gitlab.com/gorib/waffle/app"
    whttp "gitlab.com/gorib/waffle/tools/http"
)

func init() {
    di.Wire[app.BaseCommand](
        func() app.BaseCommand { return app.NewCommand("http", "Start web server") },
        di.For[whttp.HttpCommand](),
    )
    di.Wire[whttp.MiddlewareList](whttp.NewMiddlewareList, di.Args(map[string]any{
        "middlewares": di.Tags[whttp.Middleware](whttp.MiddlewareTag),
    }))
    di.Wire[whttp.HttpCommand](whttp.NewHttpCommand, di.Tag(app.CommandTag))
}
```

Run `make generate`, then start the server:

```sh
./app http
./app http --listen :9000
```

## Listen address

The default address is read from the `Router` configuration. Override it at runtime with the `--listen` / `-l` flag:

```sh
./app http --listen 0.0.0.0:8080
```

## Registering routes

Implement `http.Router` (from `gitlab.com/gorib/http`) and provide it via DI. The scaffolded `api/<service>/init.go` is the conventional place to do this:

```go
package api

import (
    "gitlab.com/gorib/http"
    "mymodule/domain/user/cmd/http/user"
)

type Router struct {
    userHandler *user.UserHandler
}

func NewRouter(userHandler *user.UserHandler) *Router {
    return &Router{userHandler: userHandler}
}

func (r *Router) Register(app *fiber.App) {
    v1 := app.Group("/v1")
    v1.Get("/users/:id", r.userHandler.Get)
    v1.Post("/users", r.userHandler.Create)
}
```

Register it in DI:

```go
di.Define(api.NewRouter)
```

## Middleware

### Middleware interface

```go
type Middleware interface {
    Name() string
    Run() fiber.Handler
}
```

Implement this interface for any middleware you want to register:

```go
type AuthMiddleware struct {
    secret string
}

func NewAuthMiddleware(cfg *config.Config) *AuthMiddleware {
    return &AuthMiddleware{secret: cfg.JWTSecret}
}

func (m *AuthMiddleware) Name() string { return "auth" }

func (m *AuthMiddleware) Run() fiber.Handler {
    return func(c *fiber.Ctx) error {
        // validate JWT
        return c.Next()
    }
}
```

### Registering middleware

Tag the middleware with `http.MiddlewareTag`:

```go
di.Wire[http.Middleware](NewAuthMiddleware, di.Tag(http.MiddlewareTag))
```

### Using middleware in routes

`InitHttp()` wires a `MiddlewareList` into the DI container. Inject it wherever you register routes and look up middleware by name:

```go
func NewRouter(middlewares http.MiddlewareList, userHandler *user.UserHandler) *Router {
    return &Router{
        middlewares: middlewares,
        userHandler: userHandler,
    }
}

func (r *Router) Register(app *fiber.App) {
    v1 := app.Group("/v1", r.middlewares.Get("auth"))
    v1.Get("/users/:id", r.userHandler.Get)
}
```

`MiddlewareList.Get` panics if the named middleware was not registered, which surfaces misconfiguration at startup rather than at request time.

## MiddlewareList

```go
type MiddlewareList interface {
    Get(name string) fiber.Handler
}
```

Constructed from all `Middleware` values tagged with `http.MiddlewareTag`. If no middlewares are registered, the list is empty and `Get` panics on any lookup.

## DI tags

| Tag | Collects |
|-----|---------|
| `http.MiddlewareTag` | All `Middleware` implementations |

## Graceful shutdown

The HTTP server registers itself with the app closer automatically. When the app receives `SIGINT` or `SIGTERM`, Fiber's graceful shutdown is triggered and in-flight requests are allowed to complete.
