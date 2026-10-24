package agents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

func service(t *testing.T) string {
	t.Helper()
	return serviceIn(t, t.TempDir())
}

func serviceIn(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
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
	err := NewGrant().Run(context.Background(), "spec", "gomod")
	if err == nil || !strings.Contains(err.Error(), "not granted in spec mode") {
		t.Errorf("gomod in spec: %v", err)
	}
}

func TestGrantOfAnAreaTheModeOpensSaysSo(t *testing.T) {
	service(t)
	err := NewGrant().Run(context.Background(), "spec", "proto")
	if err == nil || !strings.Contains(err.Error(), "open in spec mode already") {
		t.Errorf("proto in spec: %v", err)
	}
}

func TestGrantListsTheAreasAModeOpensAsWellAsItsGrants(t *testing.T) {
	r := monorepoRules(t)
	spec, _ := LookupMode("spec")
	redgreen, _ := LookupMode("redgreen")

	got := listing(spec, r)
	for _, want := range []string{"Open in spec mode on their own:", "proto — ../../shared/api/proto/",
		"events — ../../shared/api/events/", "No area takes a grant in spec mode."} {
		if !strings.Contains(got, want) {
			t.Errorf("spec listing lacks %q:\n%s", want, got)
		}
	}
	got = listing(redgreen, r)
	if strings.Contains(got, "Open in") || !strings.Contains(got, "gomod — go.mod, go.sum") {
		t.Errorf("redgreen listing:\n%s", got)
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
		"    fiscal:\n      paths: [domain/payment/fiscal/]\n      grant: [spec]\n" +
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
		{[]string{"spec", fiscal}, false},
		{[]string{"spec", fiscal, "--grant", "fiscal"}, true},
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

func TestGrantListsAPathOnlyUnderTheAreaTheGuardAnswersWith(t *testing.T) {
	r := rulesAt(t, "/svc", config("api/proto", map[string]command.AreaConfig{
		"contracts": {Paths: []string{"api/proto/"}, Modes: []string{"platform"}},
	}))
	spec, _ := LookupMode("spec")
	if v, _ := r.Check(spec, "/svc/api/proto/price/service.proto", nil); v != Refused {
		t.Errorf("verdict %v, want Refused", v)
	}
	if got := listing(spec, r); strings.Contains(got, "Open in") {
		t.Errorf("spec listing opens what the guard refuses:\n%s", got)
	}
}

func TestGrantOfAnAreaOutrankedOnEveryPathSaysItOpensNothing(t *testing.T) {
	dir := service(t)
	yaml := "agents:\n  areas:\n    contracts:\n      paths: [api/proto/]\n      modes: [platform]\n"
	if err := os.WriteFile(filepath.Join(dir, "waffle.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	err := NewGrant().Run(context.Background(), "spec", "proto")
	if err == nil || !strings.Contains(err.Error(), "opens nothing") {
		t.Errorf("proto in spec: %v", err)
	}
}

func TestGuardMatchesAreasUnderARootWithWildcardsInItsPath(t *testing.T) {
	serviceIn(t, filepath.Join(t.TempDir(), "a[1]", "svc"))
	for _, p := range []string{"go.mod", "api/proto/price/service.proto"} {
		if err := NewGuard().Run(context.Background(), "redgreen", p); err == nil {
			t.Errorf("%s was allowed in redgreen mode", p)
		}
	}
}

func TestGuardSpellsAPathAsItsDirectoryListsIt(t *testing.T) {
	tmp := t.TempDir()
	dir := serviceIn(t, filepath.Join(tmp, "svc"))
	if _, err := os.Stat(filepath.Join(tmp, "SVC")); err != nil {
		t.Skip("the temporary directory is case-sensitive")
	}
	for _, d := range []string{"tests/internal", "tests/feature"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{"waffle.yaml": "proto_dir: api/proto\n", "CONTRIBUTING.md": "# svc\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(filepath.Join(tmp, "SVC"))
	cases := []struct {
		args []string
		ok   bool
	}{
		{[]string{NoMode, "Waffle.yaml"}, false},
		{[]string{"redgreen", "Go.mod"}, false},
		{[]string{"redgreen", "Tests/Internal/harness/harness.go"}, false},
		{[]string{"platform", "contributing.md"}, false},
		{[]string{"spec", "Tests/Feature/new.feature"}, true},
	}
	for _, c := range cases {
		if err := NewGuard().Run(context.Background(), c.args...); (err == nil) != c.ok {
			t.Errorf("%v: %v, want allowed=%v", c.args, err, c.ok)
		}
	}
}

func TestGuardDecidesASymlinkAsWrittenAndAsResolved(t *testing.T) {
	tmp := t.TempDir()
	for _, d := range []string{"shared/proto", "real-lib", "vendored", "svc/api", "svc/tests/feature", "svc/domain"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for link, target := range map[string]string{
		"svc/api/proto":    "../../shared/proto",
		"lib":              "real-lib",
		"svc/vendor":       "../vendored",
		"svc/domain/cases": "../tests/feature",
	} {
		if err := os.Symlink(target, filepath.Join(tmp, link)); err != nil {
			t.Fatal(err)
		}
	}
	dir := serviceIn(t, filepath.Join(tmp, "svc"))
	yaml := "agents:\n  areas:\n    lib:\n      paths: [../lib/]\n      modes: [platform]\n"
	if err := os.WriteFile(filepath.Join(dir, "waffle.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args []string
		ok   bool
	}{
		{[]string{"spec", "api/proto/price.proto"}, true},
		{[]string{"redgreen", "api/proto/price.proto"}, false},
		{[]string{"redgreen", filepath.Join(tmp, "shared/proto/price.proto")}, false},
		{[]string{"platform", "../lib/money.go"}, true},
		{[]string{"spec", "../lib/money.go"}, false},
		{[]string{"redgreen", "vendor/client.go"}, true},
		{[]string{"spec", "vendor/client.go"}, false},
		{[]string{"redgreen", "domain/cases/quote.feature"}, false},
	}
	for _, c := range cases {
		if err := NewGuard().Run(context.Background(), c.args...); (err == nil) != c.ok {
			t.Errorf("%v: %v, want allowed=%v", c.args, err, c.ok)
		}
	}
}
