# make:domain:migrate

Adds database migrations for an existing domain. Generates the migration package, the initial SQL files, and DI configuration.

## Usage

```sh
waffle make:domain:migrate <name>
```

Example:

```sh
waffle make:domain:migrate user
```

Run this after `waffle make:domain user`.

## Generated files

### Migration package

**`domain/user/cmd/migrate/main.go`**

The migration package exports a `NewMigrations()` constructor that returns the embedded SQL files:

```go
package migrate

import "embed"

//go:embed migrations/*.sql
var migrations embed.FS

func NewMigrations() embed.FS {
    return migrations
}
```

`diogen` automatically disambiguates the generated field name by prefixing it with the constructor's package alias (e.g. `userMigrateEmbedFS`) when `di.Define` is used without an explicit interface type.

### SQL migrations

**`domain/user/cmd/migrate/migrations/00000000000000_create_update_at_function.sql`**

A reusable PostgreSQL function that automatically updates the `updated_at` column on any table that uses the trigger:

```sql
-- +goose Up
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- +goose Down
DROP FUNCTION IF EXISTS update_updated_at_column;
```

**`domain/user/cmd/migrate/migrations/<timestamp>_create_users_table.sql`**

The initial table migration. The timestamp is generated at scaffold time (format `YYYYMMDDHHmmss`). The table name is the plural of the domain name:

```sql
-- +goose Up
CREATE TABLE users (
    id         BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose Down
DROP TABLE IF EXISTS users;
```

Add your domain columns to the Up section before running for the first time.

### DI configuration

Add the following inside `InitDi()` in `config/di/di.go`:

```go
import (
    "gitlab.com/gorib/waffle/tools/migrate"
    userMigrate "mymodule/domain/user/cmd/migrate"
)

// Inside InitDi():
di.Define(userMigrate.NewMigrations, di.Tag(migrate.MigrationsTag))
```

## Post-generation steps

```sh
make .tidy generate
```

## Running migrations

```sh
go run ./cmd/app migrate up
```

Or run all migration-related subcommands through the same binary:

```sh
./app migrate status
./app migrate down
./app migrate up-to 20240315143022
```

### Available subcommands

| Subcommand | Description |
|------------|-------------|
| `up` | Apply all pending migrations (default) |
| `up-by-one` | Apply the next pending migration |
| `up-to <version>` | Apply migrations up to a specific version |
| `down` | Roll back the last applied migration |
| `down-to <version>` | Roll back to a specific version |
| `redo` | Roll back then re-apply the last migration |
| `reset` | Roll back all migrations |
| `status` | Print the status of all migrations |
| `version` | Print the current migration version |

See [tools/migrate.md](../tools/migrate.md) for the full reference.

## Adding new migrations

Create SQL files in `domain/user/cmd/migrate/migrations/` following the goose naming convention:

```
<timestamp>_<description>.sql
```

Use `goose create` or create them manually. The timestamp must be monotonically increasing.
