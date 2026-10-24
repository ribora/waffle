package migrate

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/pressly/goose/v3"
)

func TestGooseRunsEveryScaffoldedStatementWhole(t *testing.T) {
	want := map[string]struct{ up, down int }{
		"_create_update_at_function.sql": {up: 1},
		"_create_users_table.sql":        {up: 2, down: 1},
	}

	scaffolded := entityMigrations("user", time.Date(2026, 10, 16, 12, 30, 0, 0, time.UTC))
	fsys := fstest.MapFS{}
	for _, m := range scaffolded {
		body, err := templates.Render(m.template, Params("user", "user", "example.com/svc"))
		if err != nil {
			t.Fatal(err)
		}
		fsys[m.name] = &fstest.MapFile{Data: body}
	}

	db := &recorder{}
	provider, err := goose.NewProvider(goose.DialectPostgres, sql.OpenDB(db), fsys, goose.WithDisableVersioning(true))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range scaffolded {
		version, err := goose.NumericComponent(m.name)
		if err != nil {
			t.Fatal(err)
		}
		up := db.applied(t, provider, version, true)
		down := db.applied(t, provider, version, false)
		if w := want[m.suffix]; len(up) != w.up || len(down) != w.down {
			t.Errorf("%s: goose runs %d statements up and %d down, want %d and %d:\n%s",
				m.name, len(up), len(down), w.up, w.down, strings.Join(append(up, down...), "\n-- next statement\n"))
		}
	}
}

func TestTheFunctionRunsBeforeTheTableInEverySecondOfAMinute(t *testing.T) {
	minute := time.Date(2026, 10, 16, 12, 30, 0, 0, time.UTC)
	for second := range 60 {
		at := minute.Add(time.Duration(second) * time.Second)
		versions := map[string]int64{}
		for _, m := range entityMigrations("user", at) {
			version, err := goose.NumericComponent(m.name)
			if err != nil {
				t.Fatal(err)
			}
			versions[m.suffix] = version
		}
		if function, table := versions["_create_update_at_function.sql"], versions["_create_users_table.sql"]; function >= table {
			t.Errorf("at %s the function is version %d and the table %d", at.Format(time.TimeOnly), function, table)
		}
	}
}

// goose's SQL parser is internal, so its split is read off the statements it
// sends to a database that takes every one.
type recorder struct{ executed []string }

func (r *recorder) applied(t *testing.T, provider *goose.Provider, version int64, up bool) []string {
	t.Helper()
	r.executed = nil
	if _, err := provider.ApplyVersion(t.Context(), version, up); err != nil {
		t.Fatalf("version %d: %v", version, err)
	}
	return r.executed
}

func (r *recorder) Connect(context.Context) (driver.Conn, error) { return r, nil }
func (r *recorder) Driver() driver.Driver                        { return r }
func (r *recorder) Open(string) (driver.Conn, error)             { return r, nil }
func (r *recorder) Prepare(string) (driver.Stmt, error)          { return nil, errors.ErrUnsupported }
func (r *recorder) Close() error                                 { return nil }
func (r *recorder) Begin() (driver.Tx, error)                    { return r, nil }
func (r *recorder) Commit() error                                { return nil }
func (r *recorder) Rollback() error                              { return nil }

func (r *recorder) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	r.executed = append(r.executed, query)
	return driver.RowsAffected(0), nil
}
