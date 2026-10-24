package outbox

import (
	"os"
	"path/filepath"
	"testing"
)

// The table exists twice — the scaffolder's copy, which the service may edit, and
// the one the reference Store is tested against. This is what keeps the two
// originals from drifting apart.
func TestTemplateMatchesTheStoreSchema(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	tpl, err := templateFiles.ReadFile(filepath.Join("templates", "migration.sql.tpl"))
	if err != nil {
		t.Fatal(err)
	}

	pattern := filepath.Join(root, "tools", "outbox", "store", "migrations", "*"+migrationSuffix)
	found, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("expected one outbox migration under %s, found %d", pattern, len(found))
	}
	schema, err := os.ReadFile(found[0])
	if err != nil {
		t.Fatal(err)
	}

	if string(tpl) != string(schema) {
		t.Errorf("the scaffolded table and the one the reference Store is tested against have drifted apart:\n"+
			"  %s\n  %s", "cmd/waffle/internal/commands/outbox/templates/migration.sql.tpl", found[0])
	}
}
