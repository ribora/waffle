# Getting Started

[Docs](README.md)

From an empty directory to a service that answers `curl` with JSON. Nothing has
to be running — no database, no broker, no docker.

## Prerequisites

- Go 1.27 or later
- `make` and `curl`
- Access to the gorib module proxy / repository

## Install the CLI

```sh
go install gitlab.com/gorib/waffle/cmd/waffle@latest
```

Run it with no arguments to see what it can do:

```
$ waffle
Available commands:
	agents                	Regenerate waffle.md with the framework's conventions and auto-detected service context
	init:service          	Initialize empty service
	make:converter        	Scaffold a goverter dto/convert.go at the given path (creates pkg/goverter/convert.go too if missing)
	make:domain           	Make domain with entity, repository and management
	make:domain:command   	Add a CLI command (app.Command) scoped to a domain
	make:domain:entity    	Add an entity with its repository to an existing domain
	make:domain:grpc      	Make grpc connector to the specified domain
	make:domain:http      	Make HTTP router for the specified domain
	make:domain:migrate   	Set up migrations to the specified domain
	make:domain:migration 	Add a new .sql migration to an existing domain
	make:domain:scribus   	Make scribus handler for the specified domain (use --batch for BatchHandler)
	make:outbox           	Enable the transactional outbox and write its table
```

## Create a service

`init:service` fills a directory that already has a `go.mod`, so the module comes
first. The last segment of the module path becomes the service name **and** a Go
package name — keep it a plain identifier (`catalog`, not `my-service`).

```sh
mkdir catalog
cd catalog
go mod init catalog
waffle init:service
```

```
$ waffle init:service
  + .env
  + .gitignore
  + Makefile
  + api/init.go
  + api/proto/.gitignore
  + cmd/app/main.go
  + config/config.go
  + config/di/di.go
  + config/di/modules.go
  + config/diogen/di.go
  + waffle
  + waffle.yaml
  + AGENTS.md
  + added tool gitlab.com/gorib/di/cmd/diogen
  + added tool gitlab.com/gorib/waffle/cmd/waffle
```

The command then runs `make .tidy generate`, which writes
`config/diogen/di_gen.go` — the generated container — so the project compiles
before you have touched anything.

```
catalog/
├── AGENTS.md              Conventions for AI coding agents (waffle-managed)
├── Makefile               generate / .tidy / build / audit / replacements
├── api/
│   ├── init.go            Service API package
│   └── proto/             Proto sources (gRPC)
├── cmd/
│   └── app/
│       └── main.go        Entry point: config.Init(), then RunUntilStop()
├── config/
│   ├── config.go          func Init() (app.App, error)
│   ├── di/
│   │   ├── di.go          Hand-written wiring: InitDi() and the //go:generate line
│   │   └── modules.go     Blank imports — the set of waffle modules
│   └── diogen/
│       ├── di.go          Hand-written adapter (InitDi → InitApp)
│       └── di_gen.go      Generated container — do not edit
├── waffle                 ./waffle → go tool waffle
├── waffle.yaml            Project conventions read by the generators
├── .env                   Local environment
└── .gitignore
```

See [init:service](cli/init-service.md) for what each file contains.

## The first run

```
$ go run ./cmd/app --help
Usage: app [-h] command
 -h, --help  Display this help message
exit status 2
```

No commands. The entry point runs whatever the container collects, and so far
nothing registers one — every command arrives with a module. (`--help` exits
non-zero by convention, which is what `exit status 2` is.)

## Add a domain

```sh
waffle make:domain --no-repository product
```

```
  + config/di/di_product_domain.go
  + domain/product/service/service.go
  + domain/product/model/product.go
  + AGENTS.md
  ! product is not in the container until something reads its management — make generate refuses it until then; add a consumer:
      waffle make:domain:http product
      waffle make:domain:grpc product
      waffle make:domain:scribus --driver <id> product <event>
      waffle make:domain:command product <command>
```

`--no-repository` stops the chain at its first link: a model and a management,
but no postgres adapter, no table and no database module — which is
why nothing has to be running. Drop the flag when you want the storage half; see
[make:domain](cli/make-domain.md).

The domain is half of something. Nothing reads its management yet, so `diogen`
refuses to put it in the container, and `make generate` fails until the next
step gives it a reader.

## Give the domain an HTTP router

```sh
waffle make:domain:http product
```

```
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

Four of those lines are not files. The command uncommented
`_ "gitlab.com/gorib/waffle/config/tools/http"` in `config/di/modules.go`,
appended that module's variables to `.env`, and pulled in the generators the
layer uses. `init:service` writes no `DAEMON_HOST` of its own: variables arrive
with the module that reads them, and the module arrives with the command that
needs it. The full per-module list is [Running locally](reference/running-locally.md).

Ask again what the binary can do:

```
$ go run ./cmd/app --help
Usage: app [-h] command
 -h, --help  Display this help message
Available commands:
	http 	Start web server
exit status 2
```

There is the `http` command. `make:domain:http` rebuilt the container as its last
step, so `config/di/di_product_http.go` is already in it. An empty list here
means that step did not happen — run `make generate` and read what it prints.

The server would start now, and answer 404 to everything: the scaffolded router
registers no routes and `ProductManagement` declares no methods. Closing that is
three edits, one new file and one line of wiring.

## Write the route

### The model gets a field

The entity is a plain struct with no storage tags — yours to edit.

```go
// domain/product/model/product.go
package model

import (
	"time"
)

type Product struct {
	Id        int
	Title     string
	CreatedAt time.Time
	UpdatedAt *time.Time
}
```

### The management gets a method

With no repository the service answers from memory. That is the only thing here
a database would change.

```go
// domain/product/service/service.go
package service

import (
	"context"
	"time"

	"catalog/domain/product/model"
)

func NewProductManagement() *productManagement {
	return &productManagement{}
}

type productManagement struct{}

func (m *productManagement) GetProduct(_ context.Context, productId int) (*model.Product, error) {
	return &model.Product{
		Id:        productId,
		Title:     "Waffle iron",
		CreatedAt: time.Now(),
	}, nil
}
```

The scaffold's `var _ = model.Product{}` line goes: it was there to keep the
import used until the first method arrived.

### The handler

The handler is its own type behind its own interface, and the response struct
with `json` tags sits next to it — never in `model/`. The router will ask for
`Routes`, not for the struct: a constructor's concrete return type is what DI
exposes, and a component test can only stand in for an interface.

```go
// domain/product/cmd/http/product.go
package http

import (
	"strconv"

	httpServer "gitlab.com/gorib/http/server"
)

type Routes interface {
	Get(src httpServer.Source) (httpServer.Renderer, error)
}

type productResponse struct {
	Id    int    `json:"id"`
	Title string `json:"title"`
}

func NewProductHandler(productManagement ProductManagement) *productHandler {
	return &productHandler{productManagement: productManagement}
}

type productHandler struct {
	productManagement ProductManagement
}

func (h *productHandler) Get(src httpServer.Source) (httpServer.Renderer, error) {
	productId, err := strconv.Atoi(src.Path("id"))
	if err != nil {
		return nil, err
	}

	product, err := h.productManagement.GetProduct(src.Context(), productId)
	if err != nil {
		return nil, err
	}

	return httpServer.OK(&productResponse{
		Id:    product.Id,
		Title: product.Title,
	}), nil
}
```

### The router declares what it needs, and registers the route

`ProductManagement` is the transport's own interface — it says what this layer
asks of the domain, and the scaffolded wiring already fills it.

```go
// domain/product/cmd/http/main.go
package http

import (
	"context"

	httpServer "gitlab.com/gorib/http/server"
	"gitlab.com/gorib/pry"

	"catalog/domain/product/model"
)

type ProductManagement interface {
	GetProduct(ctx context.Context, productId int) (*model.Product, error)
}

func NewRouter(
	productHandler Routes,
	logger pry.Logger,
) httpServer.Router {
	return &router{
		productHandler: productHandler,
		logger:         logger,
	}
}

type router struct {
	productHandler Routes
	logger         pry.Logger
}

func (r *router) Setup(mux httpServer.Mux) {
	mux.Get("/products/{id}", httpServer.Handle(r.productHandler.Get))
}
```

### The handler is registered

One line, in the file `make:domain:http` wrote:

```go
// config/di/di_product_http.go
func InitProductHttp() {
	di.Wire[httpProduct.Routes](httpProduct.NewProductHandler)
	di.Define(httpProduct.NewRouter, di.Tag(httpCommand.RouterTag))
	di.Wire[httpProduct.ProductManagement](productManagementInstance)
}
```

`di.Wire[T](constructor)` is the form every handler in a waffle service is
registered with: the container holds a `T`, the constructor keeps returning the
unexported struct. `di.Define` alone would expose `*productHandler`, which the
generated container cannot name and no test can replace.

Nothing else needs saying — `NewRouter` asks for a `Routes` and there is now
exactly one, so diogen connects them. This is a registration change, so the
container has to be rebuilt; the next command does that itself.

## See it work

```sh
make run ARGS=http
```

`run` tidies and regenerates before it starts — `.tidy generate`, the pair every
scaffolding command runs — so the binary is built from what is on disk,
including the `di.Wire` line above. That is the one failure worth designing out:
edit a registration, start a stale container, and the code looks right while
nothing happens. The subcommand travels in `ARGS` because `make run http` would
send make looking for a target called `http`.

`go run ./cmd/app http` is the same run without those two steps — then
remembering them is yours — and `make build` puts a real binary in `bin/app`.

diogen says nothing when it regenerates, so this prints nothing at all and stays
in the foreground. In another terminal:

```
$ curl -s localhost:8080/products/42
{"id":42,"title":"Waffle iron"}
```

That is the whole path, and these are the files on it:

| File | What it decides |
|------|-----------------|
| `.env` | `DAEMON_HOST` / `DAEMON_PORT` — where the server listens |
| `config/di/modules.go` | that this service has an `http` command at all |
| `config/di/di_product_http.go` | that the router is collected under `RouterTag`, and what fills `Routes` and `ProductManagement` |
| `domain/product/cmd/http/main.go` | the route table |
| `domain/product/cmd/http/product.go` | the handler, and the JSON that goes out |
| `domain/product/service/service.go` | the answer itself |

### If nothing comes back

The silence is normal: the request log is at trace level, so start with
`LOGLEVEL=trace` in `.env` and every request shows up.

```
TRC HTTP service starting
TRC GET /products/42 >> 
TRC GET /products/42 << {"id":42,"title":"Waffle iron"}
```

An unmatched path gives the framework's error body rather than an empty 404:

```
$ curl -s localhost:8080/nope
{"code":404,"message":"Not Found","payload":{},"type":"not_found"}
```

And `curl -s localhost:8080/products/abc` gives a 500, because `strconv.Atoi`
failed and a bare error has no status to map to. That is the cost of parsing by
hand; the framework's answer is to declare the request as a struct and let the
generator write the parsing — see [Input parsing](conventions/input-parsing.md)
and [the handler pattern](tools/http-handler.md).

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
| Typed requests, validation, custom tags | [conventions/input-parsing.md](conventions/input-parsing.md) |
| HTTP handler pattern, `Bind` and OpenAPI | [tools/http-handler.md](tools/http-handler.md) |
| HTTP server setup and middleware | [tools/http.md](tools/http.md) |
| A repository, a table and a real database | [cli/make-domain.md](cli/make-domain.md) |
| Database migrations | [tools/migrate.md](tools/migrate.md) |
| Application lifecycle | [core/application.md](core/application.md) |
| Writing commands | [core/commands.md](core/commands.md) |
| gRPC server setup | [tools/grpc.md](tools/grpc.md) |
| gRPC handler pattern | [tools/grpc-handler.md](tools/grpc-handler.md) |
| Event consumer | [tools/scribus.md](tools/scribus.md) |
| Dependency injection | [di/overview.md](di/overview.md) |
