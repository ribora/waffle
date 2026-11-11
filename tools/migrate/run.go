package migrate

import (
	"context"
	dbSql "database/sql"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
	modernc "modernc.org/sqlite"
)

// goose keeps its configuration in package globals and reads one filesystem, and
// the sources share one bookkeeping table, so they run as one set. Allow-missing is
// what lets a source carry a version below the highest one applied.
func run(ctx context.Context, db *dbSql.DB, sources []fs.FS, dialect, command string, args []string) error {
	if err := goose.SetDialect(dialect); err != nil {
		return err
	}
	if len(sources) == 0 {
		return nil
	}
	set, err := merge(sources)
	if err != nil {
		return err
	}
	goose.SetBaseFS(set)
	goose.SetTableName("migrations")
	return goose.RunWithOptionsContext(ctx, command, db, "migrations", args, goose.WithAllowMissing())
}

func merge(sources []fs.FS) (fs.FS, error) {
	labelled := make(map[string]fs.FS, len(sources))
	for i, source := range sources {
		labelled[fmt.Sprintf("source %d", i+1)] = source
	}
	files, err := collect(labelled)
	if err != nil {
		return nil, err
	}
	set := oneSet{files: map[string]fs.FS{"migrations": sources[0]}}
	for _, file := range files {
		set.entries = append(set.entries, file.entry)
		set.files[path.Join("migrations", file.entry.Name())] = file.source
	}
	slices.SortFunc(set.entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return set, nil
}

type oneSet struct {
	files   map[string]fs.FS
	entries []fs.DirEntry
}

func (s oneSet) Open(name string) (fs.File, error) {
	source, ok := s.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return source.Open(name)
}

func (s oneSet) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != "migrations" {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	return s.entries, nil
}

func Up(sources ...fs.FS) func(ctx context.Context, handle *dbSql.DB) error {
	return func(ctx context.Context, handle *dbSql.DB) error {
		goose.SetLogger(goose.NopLogger())
		return run(ctx, handle, sources, dialectOf(handle), "up", nil)
	}
}

// The handle's own, as the command takes the sql.Db's: sqltest hands over a
// *sql.DB, which names its driver only by type.
func dialectOf(handle *dbSql.DB) string {
	switch handle.Driver().(type) {
	case *modernc.Driver:
		return "sqlite3"
	case *mysql.MySQLDriver:
		return "mysql"
	}
	return "postgres"
}
