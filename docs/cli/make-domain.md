# make:domain

Creates a domain and its first entity: errors and a management for the domain,
then model, repository interface, postgres adapter, stored struct, DI and a table
for the entity. Every entity added later gets that second list and nothing else —
see [make:domain:entity](#a-second-entity).

## Usage

```sh
waffle make:domain [--no-repository] <name>
```

`--no-repository` stops the chain at its first link: the domain gets its errors,
its management and the model, and no adapter, table or database module. The model
is written either way, because a repository needs one and nothing needs a
repository.

Example:

```sh
waffle make:domain user
```

## Generated files

### Model

**`domain/user/model/user.go`**

The domain entity. Plain Go struct with no database tags — it represents the business concept, not the storage format:

```go
package model

type User struct {
    Id    int
    Email string
    Name  string
}
```

**`domain/user/model/errors.go`**

Typed sentinel errors for the domain, defined with `gitlab.com/gorib/failure`:

```go
package model

import "gitlab.com/gorib/failure"

var (
    ErrUserNotFound = failure.New("user not found")
    ErrUserConflict = failure.New("user conflict")
)
```

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
got — model, adapter, stored struct, DI and its own table — and appends
`AddressRepository` to the same `service.go`. The first entity of a domain and
the tenth differ in nothing.

An entity always gets a repository — that is what the command is for, and a
model on its own is eight lines you would write faster than you would invoke
anything. `--storage` picks the adapter rather than whether there is one, and
`sql` is both the default and the only value so far.

What it does not do is add the repository to the management: whether an entity's
repository belongs there is a design decision, and a read model read only by a
handler does not. The command prints the parameter to add, and `diogen` wires it
by type as soon as it is there.

### Postgres adapter

**`domain/user/adapter/postgres/user.go`**

The repository implementation using `gitlab.com/gorib/sql`. Wraps the `sql.Repository` generic and exposes domain-typed methods:

```go
package postgres

import (
    sql "gitlab.com/gorib/sql"
    "mymodule/domain/user/model"
)

type UserRepository struct {
    sql.Repository[storedUser]
}

func NewUserRepository(db sql.Db) (*UserRepository, error) {
    repo, err := sql.NewRepository[storedUser, *storedUser](db)
    if err != nil {
        return nil, err
    }
    return &UserRepository{Repository: repo}, nil
}

func (r *UserRepository) Get(ctx context.Context, id int) (*model.User, error) {
    stored, err := r.Repository.Get(ctx,
        sql.Select().From(sql.Table("users")).
            Where(criteria.And().Add("id", "eq", id)),
    )
    if err != nil {
        return nil, err
    }
    return stored.Unwrap(), nil
}
```

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

The blank `_ *model.User` field is what marks a struct as a stored struct — it names the domain type `new<X>()` accepts and `Unwrap()` returns, and it can sit anywhere in the struct. A `db` tag puts a field in the column list; `store:"-"` keeps it out.

#### Troubleshooting `store`

Unlike `diogen` and `whisk`, `store` reads the file with `go/ast` alone and never type-checks it. That means it regenerates fine in a package that does not currently compile — and equally that a wrong mapping is reported by the compiler, in `_gen.go`, rather than by the generator. It also never deletes its previous output.

| Symptom | Cause | Fix |
|---|---|---|
| `the _ field must name the model type` | the `_` field is something else — a map, a function type | name the model |
| `an embedded field cannot carry a db tag` | an embed has no name to map, so the tag went nowhere | name the field, or drop the tag |
| `fields sharing one db tag would map to one column` | `Left, Right int \`db:"side"\`` — the second name was dropped in silence | give each field its own tag |
| a syntax error in the file | reported with the parser's own message | fix the syntax first |
| no `_gen.go` appeared | the struct has no `_` field | add `_ *model.X` |
| `m.Foo undefined (type model.User has no field or method Foo)` in `_gen.go` | `store:"Foo"` names a model field that does not exist, or the field was renamed on the model | match the model field name |
| `cannot use m.X (variable of type A) as B value` in `_gen.go` | the stored field and the model field have different types | align them, or add a `convert:` |
| a column silently missing from `Scan()`/`Inserts()` | the field has no `db` tag | add one — this compiles and fails at query time |
| a model field silently zero after `Unwrap()` | the field exists on the model but not on the stored struct | add it to the stored struct |
| `Updates()` writes a column you meant to keep | only `readonly` fields are excluded | add `store:",readonly"` |
| stale `_gen.go` for a struct you renamed or deleted | `store` only writes, never cleans up | delete the file by hand |

### Migrations

**`domain/user/cmd/migrate/main.go`**

The package whose `NewMigrations()` returns the embedded SQL, collected by the
`migrate` command from every domain that has one. Its DI registration is
**`config/di/di_user_migrate.go`**.

Two SQL files come with it, both named by the minute they were created:

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
var userManagementInstance any

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

## Post-generation steps

The command does these itself: it installs `gitlab.com/gorib/sql/cmd/store`,
enables the `service/sql` and `tools/migrate` modules in `modules.go`, and runs
`make .tidy generate` so `stored_user.go` gets its mapping methods. A project
that had no database before is ready to run when the command returns.

## What to do next

- Add fields to the model and the stored struct, then `make generate`
- Add another entity: `waffle make:domain:entity user address`
- Add gRPC support: `waffle make:domain:grpc user`
- Add a migration of your own: `waffle make:domain:migration user add_status`
