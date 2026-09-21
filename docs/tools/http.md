# HTTP Tool

[Docs](../README.md) · Tools

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

This pulls in the registrations from [`config/tools/http/init.go`](../../config/tools/http/init.go): the `http` command, the listen address, the server mux, the collection of routers, and the standard middleware chain (compress, health, session, tracing, log, error, recover). Use the manual form — copy those registrations into your `InitDi()` — only when you need to override one.

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

Implement `server.Router` — a single `Setup(mux server.Mux)` method. The scaffolded router under `domain/<x>/cmd/http` is the conventional place: it registers routes and nothing else. The global middleware chain is the server's and goes on once, before any router runs.

```go
package http

import (
    "gitlab.com/gorib/http/server"
)

type router struct {
    handler *userHandler
}

func (r *router) Setup(mux server.Mux) {
    mux.Get("/v1/users/{id}", server.Handle(r.handler.Get))
    mux.Post("/v1/users", server.Handle(r.handler.Create))
}
```

The `mux` a router is handed is its own: a `mux.Use` here covers the routes below it and no neighbour's.

Route patterns use chi syntax — `{id}`, `{id:[0-9]+}`, trailing `/*` wildcard. Matching is on the decoded path, so a path parameter always arrives decoded and a `%2F` in one is a separator: a value that can carry a slash goes in a query parameter or the body, not in a segment. The [path matching section](https://gitlab.com/gorib/http#path-matching) of the `http` README has the why.

Register the router under `RouterTag`:

```go
di.Define(userHttp.NewRouter, di.Tag(httpCommand.RouterTag))
```

## One server, every domain's routes

The command collects the tag, so every domain that serves HTTP registers its own router and they all go onto one mux, in the order the tag collects them — the same shape `grpc.ServiceTag` has. Splitting a service into an `http:customer` and an `http:admin` command is a decision about audiences, not something the wiring forces on two domains.

Two things follow from sharing a mux, and both are checked before the listener opens:

- **A pattern claimed twice is an error**, naming the method, the path and both routers. chi keeps the last registration and says nothing, so left unchecked the domain that loses is simply unreachable. Two domains that both want `/users` need a prefix, and a router takes one itself: `mux.Group("/billing")` returns a sub-mux sharing the chi router and the route list. Nothing hands out a prefix per domain — a domain is not always a path.
- **An empty collection is an error** (`ErrNoRouters`). A daemon with no routers boots, listens and 404s everything; the usual cause is a domain still registering with `di.Wire[server.Router]`, which now satisfies nothing.

## OpenAPI

The `--openapi` flag derives an OpenAPI 3.1 document from the registered routes, writes it to the given path and exits without serving:

```sh
./app http --openapi api/openapi.yaml
```

The flag runs the real wiring — the router and everything it depends on is constructed — so the spec describes the same routes the server would serve.

Only routes registered with `server.Bind` reach the spec: `Bind` captures the request and response types, `server.Handle` has nothing to capture. A `Handle` route is invisible to the generator; if you documented one by hand it stays in the file and is reported as doc-only.

```go
func (r *router) Setup(mux server.Mux) {
    mux.Get("/v1/users/{id}", server.Bind(r.handler.Get))
    mux.Post("/v1/users", server.Bind(r.handler.Create))
}
```

A `Bind` handler takes the parsed request and returns a typed response:

```go
func (h *userHandler) Get(ctx context.Context, req *getRequest) (*server.Response[userResponse], error)
```

### Code owns structure, you own prose

Every run merges, including the first — it merges into an empty document, so everything in a brand-new file arrives marked. That is deliberate: the file where nothing has been described yet is the one that needs every marker.

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

### Media types

**A route can answer XML.** `server.OK` writes JSON and `server.XML` writes XML,
and the schema generator reads `encoding/xml` tags — element names, attributes,
namespaces, a `lines>line` wrapped array, `XMLName` — so one struct is described
for both serializations. The full tag mapping is the [XML
section](https://gitlab.com/gorib/http#xml) of the `http` README.

What the generator **writes** into `content` is `application/json`, and only
that. The error renderer is why: its body is an open map, `encoding/xml` cannot
marshal a map, and a document declaring XML for a route could not then be honest
about that route's error responses.

So an XML route declares its own `content` by hand, once. `Reconcile` only ever
adds `application/json` and leaves every other media type exactly where it is —
a hand-written media type survives every later run, like the rest of the prose
you own.

### Or as a test

The config the step above needs is the reason to consider the other option: do it in a test, where the config is `t.Setenv` next to the assertion instead of pipeline setup.

```go
func TestOpenAPIIsUpToDate(t *testing.T) {
    mux := server.New()
    router.Setup(mux)

    openapitest.AssertUpToDate(t, mux.Routes(), "api/openapi.yaml", "My API")
}
```

`go test` checks, `go test -openapi.update` refreshes — the same shape as every other golden here. `Reconcile` is idempotent, so an up-to-date document compares equal. The helper reconciles into a copy, so a run that reports drift does not erase what drifted, and it fails on a leftover `TODO(openapi)` marker too — the build goes red when somebody added an endpoint and described nothing.

`waffle` does not scaffold this test, and deliberately: what it contains is a decision, not something derivable. Which fakes stand in for the leaves, which routers to walk, which file each one writes — none of that follows from the code.

One `server.New()` per document. Routes accumulate in a slice shared with every group derived from a mux, so one mux walked by two routers merges two specifications into one file. A service with a command per audience — `http:customer` and `http:admin`, say — loops once per router.

How many documents to keep follows where the **prose** differs, not where the configuration does. Two platforms that share most of their handlers are one document: the generator would write both for free, a person would maintain the same descriptions twice. The day they diverge, the merge says `operation X: in doc, not in code` — so the decision arrives as a warning rather than as a surprise.

## Middleware

Middleware is a `server.Layer`: `func(next server.Handler) server.Handler`.

The chain that runs around **every** request is `httpCommand.Global` — one method, `All() []server.Layer` — and the command puts it on the mux once, before any router registers a route. It sits outside routing, so it also covers what no route matched.

`config/tools/http` answers it with `*middleware.Standard`, in this order,
outer to inner:

| | Layer | Why there |
|---|---|---|
| 1 | `compress` | outermost, so it wraps the writer everything below it writes to |
| 2 | `health` | answers and returns, so a liveness probe builds no session and opens no span |
| 3 | `session` | the correlation id everything below it logs and traces under |
| 4 | `tracing` | the `SERVER` span, and `http.server.request.duration` |
| 5 | `log` | **outside `error`, so it sees the rendered status** rather than the raw error |
| 6 | `error` | turns an error into a response body and a status |
| 7 | `recover` | **innermost, so a handler panic becomes an error `error` can render** |

The two bold rows are the load-bearing ones — the same shape as the gRPC chain,
where `status` sits inside tracing and log for the same reason ([tools/grpc](grpc.md#unary-interceptor)).
A layer a route or a router adds composes **inside** `recover`.

There are two levels of replacement, and they answer different questions:

- **One layer**, keeping the set and the order: each layer of `Standard` is a `di.Fallback` default, so `di.Wire[middleware.Session](NewMySession)` wins over the shipped one.
- **The set or the order itself**: `Standard` is a struct, so its fields are all it can express. Register your own `di.Wire[httpCommand.Global](NewMyChain)` — without `di.Fallback()`, so it drops the default — and `All()` returns whatever you want, in whatever order.

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

Apply it to everything the router serves — layers run outer→inner in registration order, and these sit inside the global chain:

```go
func (r *router) Setup(mux server.Mux) {
    mux.Use(r.auth)
    // routes ...
}
```

To place it outside the global chain instead — the usual want for auth or cors — put it in your own `Global`:

```go
func NewChain(standard *middleware.Standard, auth server.Layer) *chain { ... }

func (c *chain) All() []server.Layer {
    return append([]server.Layer{c.auth}, c.standard.All()...)
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

## Next

| | |
|---|---|
| Writing the handlers this server routes to | [tools/http-handler](http-handler.md) |
| Declaring the request as a struct instead of parsing by hand | [conventions/input-parsing](../conventions/input-parsing.md) |
| Scaffolding a domain's router and DI | [cli/make:domain:http](../cli/make-domain-http.md) |
| `RouterTag` and every other collection | [di/tags](../di/tags.md#httproutertag) |
| `Router`, `Mux`, `Source`, `Renderer` in full | [reference/interfaces](../reference/interfaces.md#toolshttp-package) |
| What has to be in `.env` for the command to start | [reference/running-locally](../reference/running-locally.md) |
| Serving the real routes in a component test | [conventions/component-harness](../conventions/component-harness.md#the-in-process-server) |
