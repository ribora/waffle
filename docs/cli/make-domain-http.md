# make:domain:http

[Docs](../README.md) · CLI scaffolding

Adds an HTTP transport to an existing domain: a router, its DI registration, a
goverter convert package, and the `http` command itself if this is the first
domain to serve one.

## Usage

```sh
waffle make:domain:http <domain>
```

Run it after `waffle make:domain <domain>` — the registration it writes wires
the router to that domain's management handle, so the handle has to exist.

```
$ waffle make:domain:http product
  + config/di/di_product_http.go
  + domain/product/cmd/http/main.go
  + domain/product/cmd/http/convert/convert.go
  + pkg/goverter/convert.go
  + added tool gitlab.com/gorib/convert/cmd/goverter
  + enabled module gitlab.com/gorib/waffle/config/tools/http
  + added to .env: DAEMON_HOST, DAEMON_PORT, HEALTH_PATH
  + added tool gitlab.com/gorib/whisk/cmd/whisk
whisk: + /path/to/catalog/whisk.yaml
```

Four of those lines are not files, and they are the point of the command:
scaffolding a layer means enabling the module that runs it, appending the
variables that module reads, and putting the generators that layer uses in
`go.mod`. Writing the router by hand leaves all four to you.

## Generated files

### The router

**`domain/<domain>/cmd/http/main.go`**

```go
package http

import (
	httpServer "gitlab.com/gorib/http/server"
	"gitlab.com/gorib/pry"
)

type ProductManagement interface {
	// Methods consumed by the routes go here.
}

func NewRouter(
	productManagement ProductManagement,
	logger pry.Logger,
) httpServer.Router {
	return &router{
		productManagement: productManagement,
		logger:            logger,
	}
}

type router struct {
	productManagement ProductManagement
	logger            pry.Logger
}

func (r *router) Setup(mux httpServer.Mux) {
	// Register routes here, e.g.:
	//   mux.Get("/product/{id}", httpServer.Handle(r.handler.Get))
}
```

Three things in it are conventions rather than boilerplate.

**`ProductManagement` is declared here, in the transport.** It says what this
layer asks of the domain, not what the domain offers — so a second transport
declares its own, and neither constrains the other. The service package never
learns that either exists.

**`NewRouter` returns `httpServer.Router`, not `*router`.** That is what puts it
in the container under an interface the `http` command can collect.

**`Setup` registers routes and nothing else.** The chain that runs around every
request is the server's — `httpCommand.Global`, put on the mux once before any
router runs. The `mux` here is this router's own, so a `mux.Use` added to it
covers the routes below it and no neighbour's. See
[tools/http](../tools/http.md#middleware).

Handlers are not scaffolded: how many there are, and which of them take a parsed
DTO, is a design decision. [tools/http-handler](../tools/http-handler.md) is the
shape they take, and [conventions/input-parsing](../conventions/input-parsing.md)
is how a request becomes a struct.

### The registration

**`config/di/di_<domain>_http.go`**

```go
func InitProductHttp() {
	di.Define(httpProduct.NewRouter, di.Tag(httpCommand.RouterTag))
	di.Wire[httpProduct.ProductManagement](productManagementInstance)
}
```

One file per registered thing, so nothing has to be merged into a file you are
also editing. `productManagementInstance` is the handle
`config/di/di_<domain>_domain.go` published when `make:domain` ran — see
[di/overview](../di/overview.md#didefine--registering-a-concrete-constructor)
for what a handle is and why the wiring splits across two files.

`di.Define` with `RouterTag` and not `di.Wire[server.Router]`: a router reaches
the server only through the collection, and a plain `di.Wire` fills a slot
nothing asks for. See [di/tags](../di/tags.md#httproutertag).

A handler you add goes in the same file, as
`di.Wire[httpProduct.Routes](httpProduct.NewProductHandler)`: the router asks for a
`Routes`, and the interface on the registration is what makes the two meet.

### The convert package

**`domain/<domain>/cmd/http/convert/convert.go`** and **`pkg/goverter/convert.go`**

Written under the default `mapper: goverter`; with `mapper: dto` in
`waffle.yaml` neither appears and conversions are yours to write. The whitelist
file is created once per project and left alone afterwards. See
[conventions/dto-conversion](../conventions/dto-conversion.md).

## What it changes outside the domain

| Change | Why |
|---|---|
| uncomments `_ "gitlab.com/gorib/waffle/config/tools/http"` in `config/di/modules.go` | the `http` command exists because a module is imported, so the first HTTP domain is what turns it on |
| appends `DAEMON_HOST`, `DAEMON_PORT`, `HEALTH_PATH` to `.env` | a variable arrives with the module that reads it — only what is missing is added, so a second run contributes nothing |
| `go get -tool` for `whisk`, and for `goverter` under the default mapper | the generators the layer uses have to be in `go.mod` before the first `//go:generate` line you write |
| runs `go tool whisk init`, writing `whisk.yaml` | whisk owns its own defaults; an existing file is left alone |
| runs `make .tidy generate` | so the container has the new registration and the project compiles when the command returns; another domain nothing reads yet fails this step, see [make:domain](make-domain.md#post-generation-steps) |

## After it runs

```
$ go run ./cmd/app --help
Usage: app [-h] command
 -h, --help  Display this help message
Available commands:
	http    	Start web server
	migrate 	Run migrations
```

`migrate` came with `make:domain`; a domain made with `--no-repository` lists `http`
alone. An empty list here means the last step did not happen — run `make generate`
and read what it prints. The server starts and answers 404 to everything until
`Setup` registers a route: the scaffold registers none, and
`ProductManagement` declares no methods.

Idempotent: an existing file is skipped without error, so re-running it after
adding a second entity costs nothing.

## Next

| | |
|---|---|
| The whole path, from empty directory to a route answering `curl` | [Getting Started](../getting-started.md) |
| Writing the handlers this router registers | [tools/http-handler](../tools/http-handler.md) |
| The server, the middleware chain, OpenAPI | [tools/http](../tools/http.md) |
| Request DTOs, validation tags, custom validators | [conventions/input-parsing](../conventions/input-parsing.md) |
| Adding gRPC to the same domain | [make:domain:grpc](make-domain-grpc.md) |
| Every other scaffolding command | [CLI overview](overview.md) |
