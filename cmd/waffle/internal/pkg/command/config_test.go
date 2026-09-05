package command

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectConfigReadsTheAllowList(t *testing.T) {
	cfg := configFrom(t, `replacements:
  allow:
    gitlab.com/gorib/sql: [Db]
    example.test/domain/service:
      - SecurityAdapter
      - UnitRepository
`)

	for _, want := range []struct{ path, name string }{
		{"gitlab.com/gorib/sql", "Db"},
		{"example.test/domain/service", "SecurityAdapter"},
		{"example.test/domain/service", "UnitRepository"},
	} {
		if !cfg.Replacements.Allowed(want.path, want.name) {
			t.Errorf("%s.%s is not allowed, and the file grants it", want.path, want.name)
		}
	}
	if cfg.Replacements.Allowed("gitlab.com/gorib/pry", "Logger") {
		t.Error("an interface the file does not name is allowed")
	}
}

// The list is the only thing standing between a test and the whole container,
// so a file that does not say what its author meant must not load.
func TestProjectConfigRejectsAMalformedAllowList(t *testing.T) {
	cases := map[string]string{
		"empty import path": "replacements:\n  allow:\n    \"\": [Db]\n",
		"no interfaces":     "replacements:\n  allow:\n    gitlab.com/gorib/sql: []\n",
		"not exported":      "replacements:\n  allow:\n    gitlab.com/gorib/sql: [db]\n",
		"not an identifier": "replacements:\n  allow:\n    gitlab.com/gorib/sql: [\"Db.Handle\"]\n",
		"unknown key":       "mocks:\n  generate:\n    gitlab.com/gorib/sql: [Db]\n",
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "go.mod", "module example.test\n\ngo 1.27\n")
			writeFile(t, root, "waffle.yaml", body)
			t.Chdir(root)

			if _, err := New("t", "", embed.FS{}, nil).ProjectConfig("example.test"); err == nil {
				t.Fatal("the file loaded")
			} else if !strings.Contains(err.Error(), "waffle.yaml") {
				t.Errorf("err = %v, want a message naming the file", err)
			}
		})
	}
}

func configFrom(t *testing.T, body string) Config {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test\n\ngo 1.27\n")
	writeFile(t, root, "waffle.yaml", body)
	t.Chdir(root)

	cfg, err := New("t", "", embed.FS{}, nil).ProjectConfig("example.test")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func writeFile(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
