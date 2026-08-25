# HTTP Handler Pattern

This page describes how to implement HTTP handlers in a waffle service using `gitlab.com/gorib/http/server`.

## Handler signature

Every route handler has the same signature:

```go
func(src server.Source) (server.Renderer, error)
```

`src` exposes the request — path/query/header/cookie accessors, the body, and the request `Context()` (which carries the session). Register handlers on the mux, wrapping each with `server.Handle`:

```go
func (r *router) Setup(mux server.Mux) {
    mux.Get("/v1/users/{id}", server.Handle(r.handler.Get))
    mux.Post("/v1/users", server.Handle(r.handler.Create))
}
```

`server.Handle` adapts the `(Renderer, error)` handler into a layer. A nil renderer with nil error yields `204 No Content`; a non-nil error is rendered by the error middleware.

## Parsing and validating requests

`server.Into` parses the request into a DTO via its whisk-generated `Parse`, then runs `Validate()` if the type defines it:

```go
req, err := server.Into[createRequest](src)
if err != nil {
    return nil, err
}
```

The error (malformed body or failed validation) propagates to the error middleware, which renders it — you don't translate it in the handler. `Into` requires `*createRequest` to have a whisk-generated `Parse(scan.Source) error`, so a DTO without codegen won't compile.

For a typed handler that receives the parsed `*Req` directly (no manual `Into` call), register it with `server.Bind` instead of `server.Handle`:

```go
func (h *userHandler) Create(ctx context.Context, req *createRequest) (*server.Response[userResponse], error)

// in Setup:
mux.Post("/v1/users", server.Bind(r.handler.Create))
```

`Bind` also records the request and response types on the route, and that record is what the OpenAPI generator reads. A route registered with `Handle` carries no types and stays out of the specification — see [OpenAPI](http.md#openapi).

## Response helpers

```go
import (
    "net/http"

    "gitlab.com/gorib/http/server"
)

// JSON response (most common)
return server.NewJsonResponse(http.StatusOK, &userResponse{Id: user.Id}), nil

// Plain text
return server.NewResponse(http.StatusOK, lo.ToPtr("ok")), nil
```

Both take a pointer to the payload (status from `net/http`). For JSON the payload is marshalled automatically.

## Local DTOs

Request and response structs with `json` tags live in the `cmd/http` layer — never in `model/`. They are local to the transport:

```go
//go:generate go tool whisk $GOFILE

package http

type createRequest struct {
    Email string `json:"email" validate:"required,email"`
    Name  string `json:"name"  validate:"required,min=2"`
}

type userResponse struct {
    Id    int    `json:"id"`
    Email string `json:"email"`
    Name  string `json:"name"`
}
```

`whisk` generates `Parse(scan.Source) error` and `Validate() error` on the request structs. It is not HTTP-specific — the same tags bind a queue message or a file — so the tag surface, the hooks and adding your own validator live in [conventions/input-parsing.md](../conventions/input-parsing.md). What HTTP contributes are the scopes: `path`, `query`, `header`, `cookie`, `form`.

```go
type getRequest struct {
    Id     int    `scan:"path=id"`
    Expand string `scan:"query=expand"`
}
```

## Full handler example

```go
package http

import (
    "context"
    "net/http"
    "strconv"

    "gitlab.com/gorib/http/server"
    "mymodule/domain/user/model"
)

type UserManagement interface {
    GetUser(ctx context.Context, id int) (*model.User, error)
    CreateUser(ctx context.Context, email, name string) (*model.User, error)
}

type userHandler struct {
    management UserManagement
}

func NewUserHandler(management UserManagement) *userHandler {
    return &userHandler{management: management}
}

func (h *userHandler) Get(src server.Source) (server.Renderer, error) {
    id, err := strconv.Atoi(src.Path("id"))
    if err != nil {
        return nil, err
    }

    user, err := h.management.GetUser(src.Context(), id)
    if err != nil {
        return nil, err
    }

    return server.NewJsonResponse(http.StatusOK, &userResponse{
        Id:    user.Id,
        Email: user.Email,
        Name:  user.Name,
    }), nil
}

func (h *userHandler) Create(src server.Source) (server.Renderer, error) {
    req, err := server.Into[createRequest](src)
    if err != nil {
        return nil, err
    }

    user, err := h.management.CreateUser(src.Context(), req.Email, req.Name)
    if err != nil {
        return nil, err
    }

    return server.NewJsonResponse(http.StatusCreated, &userResponse{
        Id:    user.Id,
        Email: user.Email,
        Name:  user.Name,
    }), nil
}
```

## Wiring in DI

The handler and router are both concrete types — use `di.Define`:

```go
di.Define(http.NewUserHandler)
di.Define(api.NewRouter)
```

The service dependency (`UserManagement`) is resolved automatically from the registered aliases. If the service struct satisfies multiple interfaces, declare them with `di.Alias` when registering the service:

```go
di.Define(service.NewUserService,
    di.Alias[httpCmd.UserManagement](),
    di.Alias[grpcCmd.UserManagement](),
)
```

## Guidelines

- **Parse request bodies with `server.Into[T](src)`** — never read `src.Body()` and unmarshal by hand.
- **No logging in handlers** — errors propagate to the framework, which logs and renders them. Log only when you need extra context specific to this handler.
- **No `json` tags in `model/`** — DTOs with json tags belong in `cmd/http`.
- **Context**: pass `src.Context()` to all service calls — it carries the session populated by the session middleware.
