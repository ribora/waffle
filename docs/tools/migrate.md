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
    "io/fs"
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

The migrate command collects every source under the tag as `[]fs.FS` and runs
them in order. Adding a domain with migrations is the one registration above —
no change to the migrate command itself.

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

The migrate tool is configured with `goose.WithAllowMissing()`. This means if a migration file is deleted after being applied, goose will not error out on subsequent runs — it will simply skip the missing version.

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
| `migrate.MigrationsTag` | All domain `Migration` (i.e. `fs.FS`) sources |

## Next

| | |
|---|---|
| Setting a domain up with migrations | [cli/make:domain:migrate](../cli/make-domain-migrate.md) |
| Adding one more `.sql` to a domain that has them | `waffle make:domain:migration <domain> <description>` — [cli/overview](../cli/overview.md#commands) |
| The table a scaffolded entity gets, and the `stored<X>` struct it pairs with | [cli/make:domain](../cli/make-domain.md#migrations) |
| `DB_DRIVER` and `DB_DSN` | [reference/environment](../reference/environment.md#configservicesql) |
| Applying the same migrations in a component test | [conventions/component-harness](https://github.com/specrig/specrig-go/blob/main/waffle/docs/harness.md#postgres) |
