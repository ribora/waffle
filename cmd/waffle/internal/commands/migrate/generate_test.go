package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command/commandtest"
)

// A migration file name carries a timestamp, so a later run gets a different name
// and would write a second migration for the same table. The existing files here
// are stamped last year on purpose: sharing a second with the run under test would
// hide the defect behind the plain name check.
func TestExistingMigrationIsNotWrittenAgain(t *testing.T) {
	p := commandtest.Project(t, "example.com/svc")

	migrationsDir := filepath.Join("domain", "user", "cmd", "migrate", "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}
	existing := []string{
		"20250101000000_create_update_at_function.sql",
		"20250101000100_create_users_table.sql",
	}
	for _, name := range existing {
		if err := os.WriteFile(filepath.Join(migrationsDir, name), []byte("-- mine\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := Generate(p, "user", "user"); err != nil {
		t.Fatal(err)
	}

	got := migrations(t, migrationsDir)
	if len(got) != len(existing) {
		t.Errorf("migrations were written again:\nbefore: %v\nafter:  %v", existing, got)
	}

	var tables int
	for _, name := range got {
		if strings.HasSuffix(name, "_create_users_table.sql") {
			tables++
		}
	}
	if tables != 1 {
		t.Errorf("%d migrations create the users table, want 1: %v", tables, got)
	}

	// The hand-edited body has to survive too.
	body, err := os.ReadFile(filepath.Join(migrationsDir, existing[1]))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "-- mine\n" {
		t.Errorf("an existing migration was overwritten:\n%s", string(body))
	}
}

func migrations(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
