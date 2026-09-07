package component

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/gorib/waffle/replacements"
)

const containerDir = "config/diogen"

func module(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())

	files := map[string]string{
		"go.mod": "module example.test\n\ngo 1.27\n",
		"config/di/di.go": "//go:generate diogen -output config/diogen/di_gen.go -pkg diogen $GOFILE\n\n" +
			"package di\n",
		filepath.Join(containerDir, "di_gen.go"): "package diogen\n\ntype Replacements struct{}\n",
	}
	for name, body := range files {
		path := filepath.FromSlash(name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func scaffolded(t *testing.T) []string {
	t.Helper()
	var found []string
	for _, name := range []string{"app_test.go", "replacements_test.go"} {
		if _, err := os.Stat(filepath.Join(filepath.FromSlash(containerDir), name)); err == nil {
			found = append(found, name)
		}
	}
	return found
}

func TestBareRunWritesNothing(t *testing.T) {
	module(t)

	if err := New().Run(t.Context()); err != nil {
		t.Fatalf("bare run: %v", err)
	}

	if got := scaffolded(t); len(got) != 0 {
		t.Fatalf("a bare run wrote %v", got)
	}
}

func TestInitWritesBothTests(t *testing.T) {
	module(t)

	if err := New().Run(t.Context(), "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	got := strings.Join(scaffolded(t), " ")
	if got != "app_test.go replacements_test.go" {
		t.Fatalf("init wrote %q", got)
	}
}

func TestInitLeavesAnExistingTestAlone(t *testing.T) {
	module(t)
	existing := filepath.Join(filepath.FromSlash(containerDir), "app_test.go")
	if err := os.WriteFile(existing, []byte("package diogen_test // mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := New().Run(t.Context(), "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	body, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "// mine") {
		t.Fatal("init overwrote a test somebody had already written")
	}
}

func TestUnknownSubcommandIsRefused(t *testing.T) {
	module(t)

	err := New().Run(t.Context(), "generate")

	if err == nil || !strings.Contains(err.Error(), "generate") {
		t.Fatalf("run: %v", err)
	}
	if got := scaffolded(t); len(got) != 0 {
		t.Fatalf("a refused run wrote %v", got)
	}
}

func TestPlanPutsTheAllowListTestBesideTheFirstContainerOnly(t *testing.T) {
	first := plan(0, replacements.Container{Dir: "config/diogen"})
	second := plan(1, replacements.Container{Dir: "config/warehouse"})

	if len(first) != 2 {
		t.Fatalf("first container: %v", first)
	}
	if len(second) != 1 {
		t.Fatalf("second container: %v", second)
	}
	if _, ok := second[filepath.Join("templates", "app_test.go.tpl")]; !ok {
		t.Fatalf("the second container lost its wiring test: %v", second)
	}
}
