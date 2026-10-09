package migrate

import (
	"context"
	dbSql "database/sql"
	"io/fs"

	"github.com/pressly/goose/v3"
)

// goose keeps its configuration in package globals, so the source is set per
// iteration. Allow-missing is what lets a second source carry a version below
// the first one's highest.
func run(ctx context.Context, db *dbSql.DB, sources []fs.FS, dialect, command string, args []string) error {
	if err := goose.SetDialect(dialect); err != nil {
		return err
	}
	for _, source := range sources {
		goose.SetBaseFS(source)
		goose.SetTableName("migrations")
		if err := goose.RunWithOptionsContext(ctx, command, db, "migrations", args, goose.WithAllowMissing()); err != nil {
			return err
		}
	}
	return nil
}

// Postgres, because sqltest speaks nothing else.
func Up(sources ...fs.FS) func(ctx context.Context, handle *dbSql.DB) error {
	return func(ctx context.Context, handle *dbSql.DB) error {
		goose.SetLogger(goose.NopLogger())
		return run(ctx, handle, sources, "postgres", "up", nil)
	}
}
