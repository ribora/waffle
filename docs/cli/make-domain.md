# make:domain

[Docs](../README.md) · CLI scaffolding

Creates a domain and its first entity: a management for the domain, then model,
repository interface, postgres adapter, stored struct, DI and a table for the
entity. Every entity added later gets that second list and nothing else —
see [make:domain:entity](#a-second-entity).

## Usage

```sh
waffle make:domain [--no-repository] <name>
```

`--no-repository` stops the chain at its first link: the domain gets its
management and the model, and no adapter, table or database module. The model
is written either way, because a repository needs one and nothing needs a
repository.

Example:

```sh
waffle make:domain user
```

## Generated files

### Model

**`domain/user/model/user.go`**

The domain entity, and the sentinel its adapter returns for a missing row. The
struct is plain Go with no database tags — it represents the business concept,
not the storage format:

```go
package model

import (
    "gitlab.com/gorib/failure"
)

var (
    ErrUserNotFound = failure.New("user is not found")
)

type User struct {
    Id    int
    Email string
    Name  string
}
```

The sentinel belongs to the entity rather than the domain: every entity declares
its own `Err<Entity>NotFound` beside its struct, the first and the tenth alike.

`failure.New` rather than `errors.New`: it registers a slug for the message, and
the slug is what the HTTP error renderer puts in the body.

```
{"code":500,"message":"user is not found","payload":{},"type":"user_is_not_found"}
```

A bare `errors.New` reaches the client as `"type":"internal_error"` — one label
for every failure a service can have. The status is a separate decision: 500
until the error is wrapped in something that carries one, or mapped by the gRPC
error handler.

### Service

**`domain/user/service/service.go`**

The management, and above it the repository interface of every entity the domain
has. One file for the interfaces, one management per domain:

```go
package service

import (
    "context"

    "gitlab.com/gorib/criteria"

    "mymodule/domain/user/model"
)

type UserRepository interface {
    Get(ctx context.Context, userId int) (*model.User, error)
    Save(ctx context.Context, user *model.User) (*model.User, error)
    List(ctx context.Context, query criteria.Expression, sort []string, page int, count int) ([]*model.User, error)
    Count(ctx context.Context, query criteria.Expression) (int, error)
}

func NewUserManagement(userRepository UserRepository) *userManagement {
    return &userManagement{userRepository: userRepository}
}

type userManagement struct {
    userRepository UserRepository
}
```

How the management's methods are split across files is yours: by entity
(`service_user.go`) or by use case (`billing.go`, `lifecycle.go`). The
scaffolder writes neither, because it cannot tell which one a domain wants.

### A second entity

`waffle make:domain:entity user address` gives `address` the same set `user`
got — model with its `ErrAddressNotFound`, adapter, stored struct, DI and its
own table — and appends
`AddressRepository` to the same `service.go`. The first entity of a domain and
the tenth differ in nothing.

An entity always gets a repository — that is what the command is for, and a
model on its own is eight lines you would write faster than you would invoke
anything. `--storage` picks the adapter rather than whether there is one, and
`sql` is both the default and the only value so far.

What it does not do is add the repository to the management: whether an entity's
repository belongs there is a design decision, and a read model read only by a
handler does not. So it ends the way `make:domain` does — `make .tidy` and
`go generate ./domain/user/...`, not the container, which `diogen` refuses while
nothing reads the new repository. The command prints the parameter to add;
`diogen` wires it by type as soon as it is there, and `make generate` then
succeeds.

### Postgres adapter

**`domain/user/adapter/postgres/user.go`**

The implementation of the `UserRepository` interface the service declared, over
`gitlab.com/gorib/sql`. It holds a `sql.Repository[storedUser]` and exposes
domain-typed methods — `Get`, `Save`, `List`, `Count`, the four the interface
names:

```go
package postgres

import (
    "context"
    "errors"

    "gitlab.com/gorib/criteria"
    "gitlab.com/gorib/failure"
    "gitlab.com/gorib/pry"
    "gitlab.com/gorib/sql"

    "mymodule/domain/user/model"
)

var (
    userTable = sql.Table("users")
)

func NewUserRepository(
    db sql.Db,
    logger pry.Logger,
) (*userRepository, error) {
    repo, err := sql.NewRepository[storedUser](db)
    if err != nil {
        return nil, err
    }
    return &userRepository{repo: repo, logger: logger}, nil
}

type userRepository struct {
    repo   sql.Repository[storedUser]
    logger pry.Logger
}

func (r *userRepository) Get(ctx context.Context, userId int) (*model.User, error) {
    stored, err := r.repo.Get(ctx, sql.Select().From(userTable).Where(criteria.And("id", "eq", userId)))
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, failure.Parse(model.ErrUserNotFound)
        }
        if ctx.Err() == nil {
            r.logger.Error(err, pry.Ctx(ctx))
        }
        return nil, failure.Parse(sql.ErrQueryFailed)
    }
    return stored.Unwrap(), nil
}
```

Four things in that shape are the convention rather than this entity's taste.

**The constructor returns the unexported `*userRepository`.** Nothing asks for
the concrete type — `di.Wire[service.UserRepository]` is what puts it in the
container — so exporting it would only offer a second way to reach it.

**The table is a package-level `sql.Table`,** built once instead of per query.

**`sql.ErrNoRows` becomes the entity's own `ErrUserNotFound`; everything else
is logged and becomes `sql.ErrQueryFailed`.** A driver error carries the query
and sometimes its parameters, and the service above has no use for either — so
the detail goes to the log and the caller gets an error it can act on. This is
the one place a repository logs; see [No logging in handlers](../tools/http-handler.md#guidelines).
A query whose context had already ended is not logged here: whoever ended it
reports it. When the caller left, the transport answers `Canceled` quietly; when
the service cancelled its own context, the `ErrQueryFailed` reaches the
transport while the request's context is still alive, and is logged there.

**A sentinel is thrown through `failure.Parse`.** `failure.New` declares it
without a stack, and `Parse` returns a copy that carries one from the place it
was thrown. `failure.Wrap` is for a throw with context, and its `%w` comes
first: with a driver error first, the error's type would be slugged from the
driver's message.

**`domain/user/adapter/postgres/stored_user.go`**

The stored struct with field mapping tags. The `store` code generator reads this file and produces `Scan()`, `Returns()`, `Updates()`, and `Inserts()` methods:

```go
package postgres

//go:generate go tool store $GOFILE

import "mymodule/domain/user/model"

type storedUser struct {
    _ *model.User

    Id    int    `db:"id"    store:",readonly,updates:Identity"`
    Email string `db:"email"`
    Name  string `db:"name"`
}
```

After `make generate`, a companion file with all generated methods is created alongside this file. It is named after the struct, not the source file: `storedUser` produces `stored_user_gen.go`.

The blank `_ *model.User` field is what marks a struct as a stored struct — it names the domain type `new<X>()` accepts and `Unwrap()` returns, and it can sit anywhere in the struct. A `db` tag puts a field in the column list; `store:"-"` keeps it out. Every field says one or the other — a field that says neither stops the run.

#### Troubleshooting `store`

Like `diogen` and `whisk`, `store` type-checks the package it reads, and its own output carries `//go:build !store` so that a stale or broken `_gen.go` cannot block the run that replaces it. A wrong mapping is therefore reported against your tag, not by the compiler against a generated line. It still never deletes its previous output.

Every field is answered for: one with neither a `db` tag nor `store:"-"` stops the run, and a model field the stored struct does not carry is **written into the struct** without tags — naming its column, or declining it with `store:"-"`, is yours to do.

| Symptom | Cause | Fix |
|---|---|---|
| `the _ field must name the model type` | the `_` field is something else — a map, a function type | name the model, or mark the struct `store:"nonew,nounwrap"` for a read model with no model type |
| `an embedded field cannot carry a db tag` | an embed has no name to map, so the tag went nowhere | name the field, or drop the tag |
| `fields sharing one db tag would map to one column` | `Left, Right int \`db:"side"\`` — the second name was dropped in silence | give each field its own tag |
| `a field needs a db tag to become a column, or store:"-" to stay out of one` | the field says nothing about itself; silence used to mean "not a column" | add the `db` tag, or `store:"-"` |
| `added to the stored struct, now give each a db tag or store:"-"` | a model field had nothing here to carry it, and the generator wrote it in | tag the written-in field, or decline it with `store:"-"` |
| `the model has no field Foo` | `store:"Foo"` names a model field that does not exist, or the field was renamed on the model | match the model field name |
| `the field is A and model field X is B` | the stored field and the model field have different types | align them, or add a `convert:` |
| `the Json payload is A and model field X is B` | the wrapper's type argument is not the model field's type | align them |
| `convert:csv needs a *string field` / `… a []string on the model` | csv maps a model `[]string` to a stored `*string`, and one of the two is something else | align them, or drop the `convert:` |
| `the type does not resolve` | the model's own package does not compile, so the field has no type to line up with | fix what that package reports first |
| `type X is not exported by Y` | a model field to be written in has a type this package cannot name | carry it by hand, or decline it with `store:"-"` |
| `"a" and "b" would both be imported as pkg` | a written-in field's type needs an import whose name is already taken | alias the existing import, or add the field by hand |
| a syntax error in a file `store` did not generate | reported with the parser's own message | fix the syntax first |
| no `_gen.go` appeared | the struct has no `_` field | add `_ *model.X` |
| `Updates()` writes a column you meant to keep | only `readonly` fields are excluded | add `store:",readonly"` |
| stale `_gen.go` for a struct you renamed or deleted | `store` only writes, never cleans up | delete the file by hand |

### Migrations

**`domain/user/cmd/migrate/main.go`**

The package whose `NewMigrations()` returns the embedded SQL, collected by the
`migrate` command from every domain that has one. Its DI registration is
**`config/di/di_user_migrate.go`**.

Two SQL files come with it, both named by the second they were created:

**`domain/user/cmd/migrate/migrations/<timestamp>_create_update_at_function.sql`**

The trigger function the tables hang on. Written once per domain and recognised
afterwards by its name, so a second entity does not get a second copy.

**`domain/user/cmd/migrate/migrations/<timestamp>_create_users_table.sql`**

The entity's table. Every entity added later brings one of these and nothing
else from this section.

### DI configuration

Registrations go in files of their own under `config/di/`, one per thing
registered, so nothing has to be merged into a file you are also editing:

**`config/di/di_user_domain.go`** — the management.

```go
var userManagementInstance di.Handle

func InitUserDomain() {
    userManagementInstance = di.Define(service.NewUserManagement)
}
```

**`config/di/di_user_user_repository.go`** — the entity's repository, and one
more like it per entity added later.

```go
func InitUserUserRepository() {
    di.Wire[service.UserRepository](postgres.NewUserRepository)
}
```

If the management has to satisfy the interfaces a transport declares, add the
aliases to its registration:

```go
di.Define(service.NewUserManagement,
    di.Alias[grpc.UserManagement](),
    di.Alias[http.UserManagement](),
)
```

Those aliases decide something else as well. A component test can replace a
registration only when every type it is exposed as is an interface, and the two
above are exposed differently: `di.Wire[service.UserRepository]` names an
interface, so the repository gets a `Replacements` field, while `di.Define` on
its own exposes whatever the constructor returns — `*userManagement`, which no
fake can be — so the management gets none.

What both constructors return is beside the point: `postgres.NewUserRepository`
returns an unexported `*userRepository` and is replaceable anyway. Aliases do not
add to the concrete exposure, they take its place, so the registration above —
`di.Define` plus two `di.Alias` — is replaceable through a field typed as both
interfaces at once.

Leaving the management without a seam is usually right: a component test drives
the service through a transport and wants the real one. Declare an interface for
it when something below it has to be cut and the repository is not where the cut
belongs. [Component harness](../conventions/component-harness.md) covers what may
be replaced; `gitlab.com/gorib/di` covers how a seam is derived.

## Post-generation steps

The command does these itself: it installs `gitlab.com/gorib/sql/cmd/store`,
enables the `service/sql` and `tools/migrate` modules in `modules.go`, runs
`make .tidy`, and runs `go generate ./domain/user/...` so `stored_user.go` gets
its mapping methods.

It does not rebuild the container. Nothing reads the management until a
transport does, and `diogen` refuses a registration nothing reads:

```
diogen: nothing reads mymodule/domain/user/service.NewUserManagement (config/di/di_user_domain.go:12), and no di.Entry names it — declare it with di.Entry[T]() if it is built for itself, or remove it
```

That is what `make generate` prints until the first of the commands below. The
previous container stays in place, so the project still compiles; the domain
joins it with its first consumer.

## What to do next

- Add fields to the model and the stored struct, then `make generate`
- Add another entity: `waffle make:domain:entity user address`
- Serve it over HTTP: `waffle make:domain:http user` — [make:domain:http](make-domain-http.md)
- Serve it over gRPC: `waffle make:domain:grpc user` — [make:domain:grpc](make-domain-grpc.md)
- Consume events into it: `waffle make:domain:scribus --driver amqp user order_paid` — [tools/scribus](../tools/scribus.md)
- Add a migration of your own: `waffle make:domain:migration user add_status`
- Give it a CLI subcommand: `waffle make:domain:command user recalc` — [core/commands](../core/commands.md)
