package agents

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command/commandtest"
)

// make generate runs this on every build, so a second run must report nothing.
func TestGenerateReportsOnlyAChange(t *testing.T) {
	p := commandtest.Project(t, "example.com/svc")
	for i, want := range []bool{true, false} {
		changed, err := Generate(p, true)
		if err != nil {
			t.Fatal(err)
		}
		if changed != want {
			t.Errorf("run %d: changed=%v, want %v", i+1, changed, want)
		}
	}
	if err := os.Remove(".cursorrules"); err != nil {
		t.Fatal(err)
	}
	if changed, _ := Generate(p, true); !changed {
		t.Error("a missing symlink was restored without reporting it")
	}
}

func TestWaffleMdIsWrittenAndAGENTSmdBringsItIn(t *testing.T) {
	stub := "# AGENTS.md\n\nInstructions for AI coding agents working in svc.\n\n" + reference + "\n"
	for name, c := range map[string]struct{ before, after string }{
		"none":                      {"", stub},
		"the one waffle once wrote": {"# AGENTS.md\n\nThis file is\n" + managed + " — hand edits are lost.\n\n## Rule #11\n", stub},
		"the service's, without":    {"# AGENTS.md\n\nOurs.\n", "# AGENTS.md\n\nOurs.\n\n" + reference + "\n"},
		"the service's, with":       {"# AGENTS.md\n\n" + reference + "\n\nOurs.\n", "# AGENTS.md\n\n" + reference + "\n\nOurs.\n"},
	} {
		t.Run(name, func(t *testing.T) {
			p := commandtest.Project(t, "example.com/svc")
			if c.before != "" {
				if err := os.WriteFile(p.Path("AGENTS.md"), []byte(c.before), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Generate(p, true); err != nil {
				t.Fatal(err)
			}
			if got, _ := os.ReadFile(p.Path("AGENTS.md")); string(got) != c.after {
				t.Errorf("AGENTS.md:\n%s\nwant\n%s", got, c.after)
			}
			waffle, err := os.ReadFile(p.Path("waffle.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(waffle), "# waffle.md\n") || strings.Contains(string(waffle), "Rule #11") {
				t.Errorf("waffle.md:\n%s", waffle[:200])
			}
		})
	}
}

func TestScanModulesSkipsCommentedImports(t *testing.T) {
	p := inModule(t, `package di

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
	if got := scanModules(p); !slices.Equal(got, want) {
		t.Errorf("modules = %v, want %v", got, want)
	}
}

func TestScanModulesTakesOnlyBlankWaffleImports(t *testing.T) {
	p := inModule(t, `package di

import (
	"fmt"

	di "gitlab.com/gorib/waffle/config/app"
	_ "gitlab.com/gorib/waffle/tools/http"
	_ "example.com/other/config/tools/grpc"
)

var _ = fmt.Sprint
`)

	if got := scanModules(p); len(got) != 0 {
		t.Errorf("modules = %v, want none", got)
	}
}

func TestScanModulesWithoutTheFile(t *testing.T) {
	p := inModule(t, "")

	if got := scanModules(p); got != nil {
		t.Errorf("modules = %v, want nil", got)
	}
}

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
	if got := scanEnvVars(".", nil); !slices.Equal(got, want) {
		t.Errorf("vars = %v, want %v", got, want)
	}
}

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
	if got := scanEnvVars(".", nil); !slices.Equal(got, want) {
		t.Errorf("vars = %v, want %v", got, want)
	}
}

func TestScanEnvVarsTakesWhatTheModulesRead(t *testing.T) {
	inConfig(t, `package config

import "gitlab.com/gorib/env"

func Init() { _ = env.Value("OWN", "") }
`)

	want := []string{"DB_DRIVER", "DB_DSN", "OWN", "RABBIT_DSN", "RABBIT_EXCHANGE"}
	if got := scanEnvVars(".", []string{"service/amqp", "service/sql", "app"}); !slices.Equal(got, want) {
		t.Errorf("vars = %v, want %v", got, want)
	}
}

// Half-written code is the normal state of a tree being edited, and AGENTS.md
// is not where a syntax error should surface.
func TestScanEnvVarsSkipsWhatDoesNotParse(t *testing.T) {
	inConfig(t, "package config\n\nfunc Init() { this is not go")

	if got := scanEnvVars(".", nil); got != nil {
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

func inModule(t *testing.T, body string) *command.Project {
	t.Helper()
	p := commandtest.Project(t, "example.com/svc")
	if body != "" {
		if err := os.MkdirAll(p.Path("config", "di"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p.Path(command.ModulesFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return p
}
