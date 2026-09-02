# make:domain

Generates the full domain layer for a new business entity: model, errors, service, postgres adapter, and DI configuration.

## Usage

```sh
waffle make:domain <name>
```

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

**`domain/user/service/user.go`**

The business logic layer. Accepts a repository interface and implements use cases:

```go
package service

import (
    "context"
    "mymodule/domain/user/model"
)

type UserRepository interface {
    Get(ctx context.Context, id int) (*model.User, error)
    Create(ctx context.Context, user *model.User) (*model.User, error)
}

type UserService struct {
    repo UserRepository
}

func NewUserService(repo UserRepository) *UserService {
    return &UserService{repo: repo}
}
```

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

### DI configuration

The generated registrations are added inside `InitDi()` in `config/di/di.go`. The repository is wired to its interface with `di.Wire`, and the service is registered with `di.Define` (with `di.Alias` entries for any additional interfaces it must satisfy):

```go
di.Wire[service.UserRepository](postgres.NewUserRepository)
di.Define(service.NewUserService)
```

If the service implements multiple interfaces used by transport layers, add aliases:

```go
di.Define(service.NewUserService,
    di.Alias[grpc.UserManagement](),
    di.Alias[http.UserManagement](),
)
```

## Post-generation steps

```sh
go get gitlab.com/gorib/sql/cmd/store
make .tidy generate
```

The `store` tool is installed and `make generate` runs it against `stored_user.go` to produce the mapping methods.

## What to do next

- Add fields to the model and stored struct
- Implement additional repository methods
- Add gRPC support: `waffle make:domain:grpc user`
- Add migrations: `waffle make:domain:migrate user`
