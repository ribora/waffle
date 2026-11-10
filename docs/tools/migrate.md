# Migrate Tool

[Docs](../README.md) · Tools

The migrate tool wraps [goose v3](https://github.com/pressly/goose) into a waffle `Command`. It supports multiple `fs.FS` migration sources, so each domain can ship its own SQL files and they are all applied in a single run.

## Setup

Add a single blank import to `config/di/modules.go`:

```go
// config/di/modules.go
package di

import (
    _ "gitlab.com/gorib/waffle/config/tools/migrate"
    // ... other modules ...
)
```

This pulls in the registrations from [`config/tools/migrate/init.go`](../../config/tools/migrate/init.go), which is equivalent to writing them yourself:

```go
import (
    "gitlab.com/gorib/di"
    "gitlab.com/gorib/waffle/app"
    "gitlab.com/gorib/waffle/tools/migrate"
)

// Inside InitDi():
di.Wire[app.BaseCommand](
    func() app.BaseCommand { return app.NewCommand("migrate", "Run migrations") },
    di.For[migrate.MigrateCommand](),
)
di.Wire[migrate.MigrateCommand](migrate.NewMigrateCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
    "migrations": di.Tags(migrate.MigrationsTag),
}))
```

The goose dialect comes from the `sql.Db` itself — `Driver().Dialect()` — so it
always matches the connection the command runs against, whichever alias
(`pgx`, `pgsql`, `sqlite3`, …) selected it.

Run `make generate`, then run migrations:

```sh
./app migrate
# or explicitly
./app migrate up
```

## Migration sources

Each domain's migrate package exports `NewMigrations()` returning the embedded SQL files:

```go
// domain/user/cmd/migrate/main.go
package migrate

import (
    "embed"
)

//go:embed migrations/*
var migrations embed.FS

func NewMigrations() embed.FS {
    return migrations
}
```

`waffle make:domain:migrate` and `make:domain` both write the registration —
one file per domain:

```go
// config/di/di_user_migrate.go
di.Define(userMigration.NewMigrations, di.Tag(migrate.MigrationsTag))
```

A source is reached only through the tag, so it needs no interface of its own,
and registering `embed.FS` concretely in twenty domains is twenty distinct
container fields: each is named after its import alias and the type.

The migrate command collects every source under the tag as `[]fs.FS` and runs them
as one set, in version order. One domain's versions say nothing about another's, so
a domain's migrations must apply without another domain's having run. Adding a
domain with migrations is the one registration above — no change to the migrate
command itself.

## Database connection

By default the migrate command uses the `sql.Db` from the DI container — `DB_DRIVER` and `DB_DSN`, read by `config/service/sql`. Override it at runtime with the `--connection` / `-c` flag:

```sh
./app migrate --connection "postgres://user:pass@localhost:5432/mydb?sslmode=disable" up
```

The flag value is parsed as a standard DSN: `driver://user:pass@host:port/dbname`.
Its scheme selects the driver, and the dialect follows from that connection.

## Subcommands

Pass the goose subcommand as a positional argument after `migrate`:

```sh
./app migrate up
./app migrate status
./app migrate down
./app migrate up-to 20240101120000
```

| Subcommand | Description |
|------------|-------------|
| `up` | Apply all pending migrations (default if omitted) |
| `up-by-one` | Apply only the next pending migration |
| `up-to <version>` | Apply up to and including the specified version |
| `down` | Roll back the last applied migration |
| `down-to <version>` | Roll back down to the specified version |
| `redo` | Roll back then re-apply the last migration |
| `reset` | Roll back all migrations |
| `status` | Print the applied/pending status of every migration |
| `version` | Print the current schema version |

The sources are one set to every subcommand: `down` rolls back the last applied
migration whichever source ships it, and `status` lists each migration once. A
version two sources ship runs once, so they must ship the same file; different
contents under one version stop the command before anything runs.

## Migration table

Waffle configures goose to use `migrations` as the tracking table name (instead of goose's default `goose_db_version`). This is fixed and cannot be overridden.

## SQL file format

Migrations follow the standard goose annotation format:

```sql
-- +goose Up
CREATE TABLE users (
    id         BIGSERIAL PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS users;
```

File naming: `<timestamp>_<description>.sql`  
Example: `20240315143022_create_users_table.sql`

## Missing migrations

The migrate tool is configured with `goose.WithAllowMissing()`: a version below the highest one applied still runs if it has not run yet, which is what lets a second source carry versions below the first one's highest.

A file deleted after being applied is a different case. `up` does not notice it, and `down` fails with `no current version found` once that version is the current one.

## File layout

```
domain/user/cmd/migrate/
├── main.go              NewMigrations() — embeds the SQL directory
└── migrations/
    ├── 20240315143022_create_users_table.sql
    └── 20240320091500_add_users_email_index.sql
```

`main.go` is the only Go file needed. The `migrations/` subdirectory holds all SQL files for this domain.

## DI tags

| Tag | Collects |
|-----|---------|
| `migrate.MigrationsTag` | All domain `fs.FS` sources |

## Testing

`migrate.Up(sources...)` applies the sources in a test, postgres only. It returns the function `sqltest.Migrations` takes:

```go
postgres := sqltest.New(
    sqltest.Env("DB_DSN"),
    sqltest.Migrations(migrate.Up(userMigration.NewMigrations(), billingMigration.NewMigrations())),
)
```

A domain's migrations must apply alone: `migrate.Up` over one source and an empty database is what shows they do.

`migrate.CheckShared` refuses two different files under one version, as the command does, and names the sources, so a test finds it before any database does:

```go
err := migrate.CheckShared(map[string]fs.FS{
    "user":    userMigration.NewMigrations(),
    "billing": billingMigration.NewMigrations(),
})
```

## Next

| | |
|---|---|
| Setting a domain up with migrations | [cli/make:domain:migrate](../cli/make-domain-migrate.md) |
| Adding one more `.sql` to a domain that has them | `waffle make:domain:migration <domain> <description>` — [cli/overview](../cli/overview.md#commands) |
| The table a scaffolded entity gets, and the `stored<X>` struct it pairs with | [cli/make:domain](../cli/make-domain.md#migrations) |
| `DB_DRIVER` and `DB_DSN` | [reference/environment](../reference/environment.md#configservicesql) |
