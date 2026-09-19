package agents

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The scaffolded modules.go is mostly commented-out lines, so a service with
// five modules listed twelve and an agent read http and scribus as available.
func TestScanModulesSkipsCommentedImports(t *testing.T) {
	inModule(t, `package di

import (
	_ "gitlab.com/gorib/waffle/config/app"

	// logger
	_ "gitlab.com/gorib/waffle/config/service/pry"

	// uncomment if your service uses redis
	// _ "gitlab.com/gorib/waffle/config/service/redis"

	_ "gitlab.com/gorib/waffle/config/tools/grpc"
	// uncomment if your service uses http server
	// _ "gitlab.com/gorib/waffle/config/tools/http"
	// CHOOSE ONLY ONE VARIANT
	// _ "gitlab.com/gorib/waffle/config/tools/scribus/batch"
	// _ "gitlab.com/gorib/waffle/config/tools/scribus/single"
)
`)

	want := []string{"app", "service/pry", "tools/grpc"}
	if got := scanModules(); !slices.Equal(got, want) {
		t.Errorf("modules = %v, want %v", got, want)
	}
}

// A named import is not a module registration, and the file holds nothing else
// of waffle's — so neither may be reported.
func TestScanModulesTakesOnlyBlankWaffleImports(t *testing.T) {
	inModule(t, `package di

import (
	"fmt"

	di "gitlab.com/gorib/waffle/config/app"
	_ "gitlab.com/gorib/waffle/tools/http"
	_ "example.com/other/config/tools/grpc"
)

var _ = fmt.Sprint
`)

	if got := scanModules(); len(got) != 0 {
		t.Errorf("modules = %v, want none", got)
	}
}

func TestScanModulesWithoutTheFile(t *testing.T) {
	inModule(t, "")

	if got := scanModules(); got != nil {
		t.Errorf("modules = %v, want nil", got)
	}
}

// The readers differ in how they are written, not in where the key sits, and
// the regexp this replaced saw four of the nine.
func TestScanEnvVarsReadsEveryCallShape(t *testing.T) {
	inConfig(t, `package config

import "gitlab.com/gorib/env"

func Init() {
	_ = env.Value("PLAIN", "")
	_ = env.NeedValue[string]("INSTANTIATED")
	_ = env.Array[string]("ARRAY", nil)
	_ = env.Json[map[string]string]("JSON", nil)
	_, _ = env.MustValue[int]("MUST")
	_ = env.NeedJson[int]("NEED_JSON")
	// _ = env.Value("COMMENTED", "")
}
`)

	want := []string{"ARRAY", "INSTANTIATED", "JSON", "MUST", "NEED_JSON", "PLAIN"}
	if got := scanEnvVars(); !slices.Equal(got, want) {
		t.Errorf("vars = %v, want %v", got, want)
	}
}

// The qualifier is whatever the file called the import, and a selector that
// only looks like one belongs to something else.
func TestScanEnvVarsFollowsTheImport(t *testing.T) {
	inConfig(t, `package config

import environment "gitlab.com/gorib/env"

type fake struct{}

func (fake) Value(key string, fallback string) string { return fallback }

var env = fake{}

func Init() {
	_ = environment.Value("REAL", "")
	_ = env.Value("IMPOSTOR", "")
}
`)

	want := []string{"REAL"}
	if got := scanEnvVars(); !slices.Equal(got, want) {
		t.Errorf("vars = %v, want %v", got, want)
	}
}

// Half-written code is the normal state of a tree being edited, and AGENTS.md
// is not where a syntax error should surface.
func TestScanEnvVarsSkipsWhatDoesNotParse(t *testing.T) {
	inConfig(t, "package config\n\nfunc Init() { this is not go")

	if got := scanEnvVars(); got != nil {
		t.Errorf("vars = %v, want nil", got)
	}
}

func inConfig(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config", "init.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
}

// body empty writes no file, for the case where the project has no modules.go.
func inModule(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	if body != "" {
		if err := os.MkdirAll(filepath.Join(dir, "config", "di"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config", "di", "modules.go"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
}
