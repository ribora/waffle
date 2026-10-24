package command

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const scribusDriver = "gitlab.com/gorib/waffle/config/tools/scribus/amqp"

func withModules(t *testing.T, modules string) *Project {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll(filepath.Join("config", "di"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ModulesFile, []byte(modules), 0644); err != nil {
		t.Fatal(err)
	}
	return &Project{root: dir}
}

func TestEnableModuleAddsEnvKeys(t *testing.T) {
	p := withModules(t, `package di

import (
	_ "gitlab.com/gorib/waffle/config/app"
	// _ "gitlab.com/gorib/waffle/config/tools/http"
	// _ "gitlab.com/gorib/waffle/config/service/sql"
)
`)
	if err := os.WriteFile(".env", []byte("SERVICE=demo\nDAEMON_PORT=9999\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := p.EnableModule("gitlab.com/gorib/waffle/config/tools/http"); err != nil {
		t.Fatal(err)
	}
	if err := p.EnableModule("gitlab.com/gorib/waffle/config/service/sql"); err != nil {
		t.Fatal(err)
	}

	env, err := os.ReadFile(".env")
	if err != nil {
		t.Fatal(err)
	}
	body := string(env)

	for _, key := range []string{"DAEMON_HOST", "HEALTH_PATH", "DB_DRIVER", "DB_DSN"} {
		if !strings.Contains(body, key+"=") {
			t.Errorf("enabling the modules did not add %s to .env:\n%s", key, body)
		}
	}
	if strings.Count(body, "DAEMON_PORT=") != 1 {
		t.Errorf("DAEMON_PORT was already set and must not be added again:\n%s", body)
	}
	if !strings.Contains(body, "DAEMON_PORT=9999") {
		t.Errorf("an existing value was overwritten:\n%s", body)
	}

	// running it again must change nothing
	before := body
	if err := p.EnableModule("gitlab.com/gorib/waffle/config/tools/http"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(".env")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != before {
		t.Errorf("re-running changed .env:\n%s", string(after))
	}
}

// Enabling a module by hand is a normal thing to do, and it skips the step that
// writes the variables. The service then stops at startup naming one of them.
func TestHandEnabledModuleStillGetsItsEnv(t *testing.T) {
	// Already active: there is nothing left to uncomment.
	p := withModules(t, `package di

import (
	_ "gitlab.com/gorib/waffle/config/service/sql"
)
`)

	if err := p.EnableModule("gitlab.com/gorib/waffle/config/service/sql"); err != nil {
		t.Fatal(err)
	}

	env, err := os.ReadFile(".env")
	if err != nil {
		t.Fatalf("no .env was written for a module enabled by hand: %v", err)
	}
	for _, key := range []string{"DB_DRIVER", "DB_DSN"} {
		if !strings.Contains(string(env), key+"=") {
			t.Errorf("%s is missing, the service would stop at startup:\n%s", key, string(env))
		}
	}
}

// http and grpc are two commands of one binary and share DAEMON_HOST/DAEMON_PORT.
// Enabling both must not write the pair twice.
func TestSharedEnvKeysAreWrittenOnce(t *testing.T) {
	p := withModules(t, `package di

import (
	// _ "gitlab.com/gorib/waffle/config/tools/http"
	// _ "gitlab.com/gorib/waffle/config/tools/grpc"
)
`)

	for _, module := range []string{
		"gitlab.com/gorib/waffle/config/tools/http",
		"gitlab.com/gorib/waffle/config/tools/grpc",
	} {
		if err := p.EnableModule(module); err != nil {
			t.Fatal(err)
		}
	}

	env, err := os.ReadFile(".env")
	if err != nil {
		t.Fatal(err)
	}
	body := string(env)

	for _, key := range []string{"DAEMON_HOST", "DAEMON_PORT"} {
		if n := assignments(body, key); n != 1 {
			t.Errorf("%s is assigned %d times, want 1:\n%s", key, n, body)
		}
	}
	if !strings.Contains(body, "HEALTH_PATH=") {
		t.Errorf("the http-only key is missing:\n%s", body)
	}
}

// Seven of nine services had stripped the commented lines, and the module stayed
// off while the command exited 0.
func TestEnablingAModuleWithoutALineIsAnError(t *testing.T) {
	p := withModules(t, `package di

import (
	_ "gitlab.com/gorib/waffle/config/app"
)
`)

	err := p.EnableModule("gitlab.com/gorib/waffle/config/tools/http")
	if err == nil {
		t.Fatal("a module with no line in modules.go was reported enabled")
	}
	if !strings.Contains(err.Error(), `_ "gitlab.com/gorib/waffle/config/tools/http"`) {
		t.Errorf("the error does not name the line to add: %v", err)
	}
}

func assignments(env, key string) int {
	n := 0
	for _, line := range strings.Split(env, "\n") {
		if found, ok := assignment(line); ok && found == key {
			n++
		}
	}
	return n
}

// A line to uncomment is all the precondition asks for: the first handler of a
// service leaves it commented, every one after it finds it already enabled.
func TestACommentedOrActiveLineIsAccepted(t *testing.T) {
	for _, line := range []string{
		`	// _ "gitlab.com/gorib/waffle/config/tools/scribus/amqp"`,
		`	_ "gitlab.com/gorib/waffle/config/tools/scribus/amqp"`,
	} {
		p := withModules(t, "package di\n\nimport (\n"+line+"\n)\n")

		if err := p.RequireModules(scribusDriver); err != nil {
			t.Errorf("RequireModules with %q: %v", strings.TrimSpace(line), err)
		}
	}
}

// The case that made this a precondition: the miss would surface at the end of
// the command, after every file was written.
func TestTheRefusalNamesEveryLineToAdd(t *testing.T) {
	p := withModules(t, `package di

import (
	_ "gitlab.com/gorib/waffle/config/app"
	_ "gitlab.com/gorib/waffle/config/tools/scribus/single"
)
`)

	err := p.RequireModules(scribusDriver, "gitlab.com/gorib/waffle/config/tools/scribus/single", MigrationModules[0])

	if err == nil {
		t.Fatal("a project with no driver line was accepted")
	}
	for _, want := range []string{`_ "` + scribusDriver + `"`, `_ "` + MigrationModules[0] + `"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name the line %s: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "scribus/single") {
		t.Errorf("the error names a line that is there: %v", err)
	}
}

// EnableModule used to be reached last, after eleven files and a go.mod edit.
func TestRequireModules(t *testing.T) {
	p, _ := scaffoldIn(t)

	if err := p.RequireModules(); err == nil {
		t.Error("a project without config/di/modules.go passed the precondition")
	}

	if err := os.MkdirAll(filepath.Join("config", "di"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ModulesFile, []byte("package di\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := p.RequireModules(); err != nil {
		t.Errorf("err = %v", err)
	}
}

func TestActiveModulesSkipsCommentedLines(t *testing.T) {
	p := withModules(t, `package di

import (
	_ "gitlab.com/gorib/waffle/config/app"
	// _ "gitlab.com/gorib/waffle/config/tools/scribus/batch"
	_ "gitlab.com/gorib/waffle/config/tools/scribus/single" // the mode
)
`)

	active, err := p.ActiveModules()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"gitlab.com/gorib/waffle/config/app", "gitlab.com/gorib/waffle/config/tools/scribus/single"}
	if !slices.Equal(active, want) {
		t.Errorf("active = %v, want %v", active, want)
	}
}
