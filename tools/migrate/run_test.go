package migrate

import (
	"context"
	dbSql "database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"

	"gitlab.com/gorib/sql"
)

func createTable(name string) string {
	return fmt.Sprintf("-- +goose Up\ncreate table %[1]s (id int);\n-- +goose Down\ndrop table %[1]s;\n", name)
}

func sqlite(t *testing.T) *dbSql.DB {
	t.Helper()
	goose.SetLogger(goose.NopLogger())
	db, err := sql.NewDb("sqlite3", filepath.Join(t.TempDir(), "migrations.db"))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := db.DbSql()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	return handle
}

func applied(t *testing.T, db *dbSql.DB) []int {
	t.Helper()
	rows, err := db.Query("select version_id from migrations where version_id > 0 order by version_id")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	versions := []int{}
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			t.Fatal(err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return versions
}

func TestCommandsTreatTheSourcesAsOneSet(t *testing.T) {
	orders := source(map[string]string{"1_orders.sql": createTable("orders"), "3_lines.sql": createTable("lines")})
	billing := source(map[string]string{"2_invoices.sql": createTable("invoices"), "4_payments.sql": createTable("payments")})

	for name, tc := range map[string]struct {
		commands [][]string
		want     []int
	}{
		"up":        {[][]string{{"up"}}, []int{1, 2, 3, 4}},
		"up-by-one": {[][]string{{"up-by-one"}}, []int{1}},
		"up-to":     {[][]string{{"up-to", "2"}}, []int{1, 2}},
		"down":      {[][]string{{"up"}, {"down"}}, []int{1, 2, 3}},
		"down-to":   {[][]string{{"up"}, {"down-to", "2"}}, []int{1, 2}},
		"redo":      {[][]string{{"up"}, {"redo"}}, []int{1, 2, 3, 4}},
		"reset":     {[][]string{{"up"}, {"reset"}}, []int{}},
	} {
		t.Run(name, func(t *testing.T) {
			db := sqlite(t)
			for _, command := range tc.commands {
				if err := run(context.Background(), db, []fs.FS{orders, billing}, "sqlite3", command[0], command[1:]); err != nil {
					t.Fatalf("%v: %v", command, err)
				}
			}
			if got := applied(t, db); !slices.Equal(got, tc.want) {
				t.Errorf("applied %v, want %v", got, tc.want)
			}
		})
	}
}

type countingLogger struct{ printed int }

func (l *countingLogger) Fatalf(string, ...any) {}
func (l *countingLogger) Printf(string, ...any) { l.printed++ }

func TestVersionAnswersOnceForAllTheSources(t *testing.T) {
	db := sqlite(t)
	sources := []fs.FS{
		source(map[string]string{"1_orders.sql": createTable("orders")}),
		source(map[string]string{"2_invoices.sql": createTable("invoices")}),
	}
	if err := run(context.Background(), db, sources, "sqlite3", "up", nil); err != nil {
		t.Fatal(err)
	}

	logger := &countingLogger{}
	goose.SetLogger(logger)
	t.Cleanup(func() { goose.SetLogger(goose.NopLogger()) })
	if err := run(context.Background(), db, sources, "sqlite3", "version", nil); err != nil {
		t.Fatal(err)
	}
	if logger.printed != 1 {
		t.Errorf("version printed %d lines, want 1", logger.printed)
	}
}

func TestAVersionBelowTheHighestStillRuns(t *testing.T) {
	db := sqlite(t)
	orders := source(map[string]string{"3_orders.sql": createTable("orders")})
	billing := source(map[string]string{"2_invoices.sql": createTable("invoices")})

	if err := run(context.Background(), db, []fs.FS{orders}, "sqlite3", "up", nil); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), db, []fs.FS{orders, billing}, "sqlite3", "up", nil); err != nil {
		t.Fatal(err)
	}
	if got, want := applied(t, db), []int{2, 3}; !slices.Equal(got, want) {
		t.Errorf("applied %v, want %v", got, want)
	}
}

func TestAVersionShippedTwiceVerbatimRunsOnce(t *testing.T) {
	db := sqlite(t)
	shared := createTable("orders")
	sources := []fs.FS{
		source(map[string]string{"1_orders.sql": shared}),
		source(map[string]string{"1_orders_copy.sql": shared, "2_invoices.sql": createTable("invoices")}),
	}

	if err := run(context.Background(), db, sources, "sqlite3", "up", nil); err != nil {
		t.Fatal(err)
	}
	if got, want := applied(t, db), []int{1, 2}; !slices.Equal(got, want) {
		t.Errorf("applied %v, want %v", got, want)
	}
}

func TestRunRefusesAVersionShippedTwiceDifferently(t *testing.T) {
	db := sqlite(t)
	sources := []fs.FS{
		source(map[string]string{"1_orders.sql": createTable("orders")}),
		source(map[string]string{"1_invoices.sql": createTable("invoices")}),
	}

	err := run(context.Background(), db, sources, "sqlite3", "up", nil)
	if err == nil {
		t.Fatal("two different migrations under one version were run")
	}
	for _, want := range []string{"1_orders.sql", "1_invoices.sql"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %s: %v", want, err)
		}
	}
}

func TestRunReportsASourceWithoutMigrations(t *testing.T) {
	err := run(context.Background(), sqlite(t), []fs.FS{source(nil)}, "sqlite3", "up", nil)
	if err == nil {
		t.Fatal("a source without migrations was accepted")
	}
}

// A run that never calls SetDialect accepts any driver and then builds goose's
// bookkeeping table with postgres syntax, whatever the service is connected to.
// Rejecting an unknown dialect is how that call is visible from out here.
func TestRunSetsTheDialect(t *testing.T) {
	err := run(context.Background(), nil, nil, "nonsense", "up", nil)
	if err == nil {
		t.Fatal("run accepted an unknown dialect")
	}
	if !strings.Contains(err.Error(), "nonsense") {
		t.Fatalf("the error does not name the dialect: %v", err)
	}
}

func TestGooseKnowsEveryDialect(t *testing.T) {
	for _, name := range []string{"postgres", "pgx", "pgsql", "mysql", "sqlite", "sqlite3"} {
		t.Run(name, func(t *testing.T) {
			db, err := sql.NewDb(name, "")
			if err != nil {
				t.Fatal(err)
			}
			if err = run(context.Background(), nil, nil, db.Driver().Dialect(), "up", nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}
