# HTTP Handler Pattern

[Docs](../README.md) · Tools

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

`scan.Into` parses the request into a DTO via its whisk-generated `Parse`, then runs `Validate()` if the type defines it:

```go
req, err := scan.Into[createRequest](src)
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

## Without the framework

The same route, written against `net/http` directly, so the trade is visible:

```go
func (h *userHandler) Create(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    req, err := parseCreateRequest(r)          // see conventions/input-parsing.md
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        _ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
        return
    }

    user, err := h.service.Create(ctx, req.Email)
    if err != nil {
        status := http.StatusInternalServerError        // and every domain error
        if errors.Is(err, model.ErrAlreadyExists) {     // mapped again, here,
            status = http.StatusConflict                // in every handler
        }
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(status)
        _ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    _ = json.NewEncoder(w).Encode(userResponse{Id: user.Id})
}
```

The framework version:

```go
func (h *userHandler) Create(ctx context.Context, req *createRequest) (*server.Response[userResponse], error) {
    user, err := h.service.Create(ctx, req.Email)
    if err != nil {
        return nil, err
    }
    return server.Created(&userResponse{Id: user.Id}), nil
}
```

The difference is not length. It is **where decisions live**:

| Decision | Without | With |
|---|---|---|
| Which status a domain error maps to | in every handler that can return it | once, in the error middleware |
| The shape of an error body | in every handler | once, in the error middleware |
| Content type and encoding | in every handler | in the response helper |
| Whether the request is valid before the service is called | wherever you remember to check | the signature — `req` cannot be unparsed |

The last row is the one that matters: the handler receives a `*createRequest`
that exists only if parsing and validation succeeded, so there is no path
through it where an invalid request reaches the service.

## Response helpers

Every one of these returns a `*server.Response[Res]` and takes a **pointer** to
the payload. The status-named ones are JSON, which is what a handler wants
almost every time:

```go
return server.OK(&userResponse{Id: user.Id}), nil        // 200
return server.Created(&userResponse{Id: user.Id}), nil   // 201
return server.Accepted(&jobResponse{Id: job.Id}), nil    // 202
return server.NoContent(), nil                           // 204, no payload
```

The rest take the status, because it is the part that varies:

| Helper | Content-Type |
|---|---|
| `server.JSON(status, body)` | `application/json` |
| `server.XML(status, body)` | `application/xml` — see [OpenAPI / media types](http.md#media-types) |
| `server.Text(status, body)` | `text/plain; charset=utf-8` |
| `server.HTML(status, body)` | `text/html; charset=utf-8` |
| `server.StreamWith(status, contentType, fn)` | yours; the body is written incrementally by `fn` |

`server.NewJsonResponse` and `server.NewResponse` are the older spelling of
`JSON` and `Text` and still work. New code takes the shorter ones.

Returning a nil renderer with a nil error also renders 204, so `NoContent()` is
for saying it on purpose rather than by omission.

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
    "strconv"

    "gitlab.com/gorib/http/server"
    "gitlab.com/gorib/whisk/scan"

    "mymodule/domain/user/model"
)

// What the router asks for, and the seam a component test stands in on.
type Routes interface {
    Get(src server.Source) (server.Renderer, error)
    Create(src server.Source) (server.Renderer, error)
}

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

    return server.OK(&userResponse{
        Id:    user.Id,
        Email: user.Email,
        Name:  user.Name,
    }), nil
}

func (h *userHandler) Create(src server.Source) (server.Renderer, error) {
    req, err := scan.Into[createRequest](src)
    if err != nil {
        return nil, err
    }

    user, err := h.management.CreateUser(src.Context(), req.Email, req.Name)
    if err != nil {
        return nil, err
    }

    return server.Created(&userResponse{
        Id:    user.Id,
        Email: user.Email,
        Name:  user.Name,
    }), nil
}
```

## Wiring in DI

The handler goes in under an interface, the router under the collection:

```go
di.Wire[userHttp.Routes](userHttp.NewUserHandler)
di.Define(userHttp.NewRouter, di.Tag(httpCommand.RouterTag))
```

Two different reasons, and neither is a style preference.

**The handler is `di.Wire[Routes]`** — the interface declared beside it above —
because `di.Define` alone would expose `*userHandler`, a name the generated
container cannot use and no component test can stand in for. The router asks
for a `Routes`, so the interface is also what makes the two meet.

**The router is `di.Define` plus the tag** because it reaches the server only
through the collection; a plain `di.Wire[server.Router]` fills a slot nothing
asks for any more. See [di/tags](../di/tags.md#httproutertag).

The service dependency (`UserManagement`) is resolved automatically from the registered aliases. If the service struct satisfies multiple interfaces, declare them with `di.Alias` when registering the service:

```go
di.Define(service.NewUserService,
    di.Alias[httpCmd.UserManagement](),
    di.Alias[grpcCmd.UserManagement](),
)
```

## Guidelines

- **Parse request bodies with `scan.Into[T](src)`** — never read `src.Body()` and unmarshal by hand.
- **No logging in handlers** — errors propagate to the framework, which logs and renders them. Log only when you need extra context specific to this handler.
- **No `json` tags in `model/`** — DTOs with json tags belong in `cmd/http`.
- **Context**: pass `src.Context()` to all service calls — it carries the session populated by the session middleware.

## Next

| | |
|---|---|
| The server these handlers are mounted on | [tools/http](http.md) |
| Every `scan` and `validate` tag, the hooks, custom validators | [conventions/input-parsing](../conventions/input-parsing.md) |
| Scaffolding the router, the handler and the convert package | [cli/make:domain:http](../cli/make-domain-http.md) |
| `Handler`, `Source`, `Renderer` and the response helpers | [reference/interfaces](../reference/interfaces.md#handler-and-layer) |
| Getting an OpenAPI document out of these routes | [tools/http](http.md#openapi) |
