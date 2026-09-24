package agents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func service(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/svc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return dir
}

func TestGuardAnswersForAPathNotYetCreated(t *testing.T) {
	dir := service(t)

	if err := NewGuard().Run(context.Background(), "spec", filepath.Join(dir, "tests/feature/new.feature")); err != nil {
		t.Errorf("a new case in spec mode was refused: %v", err)
	}
	err := NewGuard().Run(context.Background(), "spec", "domain/price/service/new.go")
	if err == nil || !strings.Contains(err.Error(), "redgreen mode, or platform mode") {
		t.Errorf("production code in spec mode: %v", err)
	}
}

// t.TempDir is under /var on macOS, which is a symlink to /private/var.
func TestGuardResolvesTheRootThroughSymlinks(t *testing.T) {
	dir := service(t)
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(dir, "waffle.yaml"), filepath.Join(resolved, "waffle.yaml")} {
		if err := NewGuard().Run(context.Background(), "platform", p); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
}

func TestGuardTakesGrants(t *testing.T) {
	service(t)
	if err := NewGuard().Run(context.Background(), "redgreen", "go.mod"); err == nil {
		t.Error("go.mod was allowed in redgreen with no grant")
	}
	if err := NewGuard().Run(context.Background(), "redgreen", "go.mod", "--grant", "gomod"); err != nil {
		t.Errorf("go.mod with the gomod grant: %v", err)
	}
	if err := NewGuard().Run(context.Background(), "redgreen", "go.mod", "--grant", "all"); err == nil {
		t.Error("a grant naming no area opened go.mod")
	}
}

func TestGrantBelongsToItsMode(t *testing.T) {
	service(t)
	if err := NewGrant().Run(context.Background(), "redgreen", "gomod"); err != nil {
		t.Errorf("gomod in redgreen: %v", err)
	}
	err := NewGrant().Run(context.Background(), "steps", "gomod")
	if err == nil || !strings.Contains(err.Error(), "not granted in steps mode") {
		t.Errorf("gomod in steps: %v", err)
	}
}

func TestGuardLeavesAPathNoAreaNamesOutsideTheService(t *testing.T) {
	service(t)
	if err := NewGuard().Run(context.Background(), "spec", filepath.Join(os.TempDir(), "elsewhere.go")); err != nil {
		t.Errorf("got %v", err)
	}
}

// An area naming no modes only adds a grant; `modes: []` closes it. The two are
// told apart by whether the key is there, so this goes through real YAML.
func TestAnAreaWithoutModesKeepsTheTable(t *testing.T) {
	dir := service(t)
	yaml := "agents:\n  areas:\n" +
		"    fiscal:\n      paths: [domain/payment/fiscal/]\n      grant: [steps]\n" +
		"    frozen:\n      paths: [domain/payment/frozen/]\n      modes: []\n"
	if err := os.WriteFile(filepath.Join(dir, "waffle.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	fiscal := "domain/payment/fiscal/fiscal.go"
	cases := []struct {
		args []string
		ok   bool
	}{
		{[]string{"redgreen", fiscal}, true},
		{[]string{NoMode, fiscal}, true},
		{[]string{"steps", fiscal}, false},
		{[]string{"steps", fiscal, "--grant", "fiscal"}, true},
		{[]string{"redgreen", "domain/payment/frozen/x.go"}, false},
		{[]string{"platform", "domain/payment/frozen/x.go"}, false},
	}
	for _, c := range cases {
		if err := NewGuard().Run(context.Background(), c.args...); (err == nil) != c.ok {
			t.Errorf("%v: %v, want allowed=%v", c.args, err, c.ok)
		}
	}
}

func TestGuardReadsAreasFromWaffleYaml(t *testing.T) {
	dir := service(t)
	yaml := "agents:\n  areas:\n    events:\n      paths: [../events/]\n      modes: [spec]\n"
	if err := os.WriteFile(filepath.Join(dir, "waffle.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	event := filepath.Join(filepath.Dir(dir), "events", "price.go")

	if err := NewGuard().Run(context.Background(), "spec", event); err != nil {
		t.Errorf("a shared event in spec mode: %v", err)
	}
	err := NewGuard().Run(context.Background(), "redgreen", event)
	if err == nil || !strings.Contains(err.Error(), "spec mode") {
		t.Errorf("a shared event in redgreen mode: %v", err)
	}
}
