package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUncommentModuleAddsEnvKeys(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	if err := os.MkdirAll(filepath.Join("config", "di"), 0755); err != nil {
		t.Fatal(err)
	}
	modules := `package di

import (
	_ "gitlab.com/gorib/waffle/config/app"
	// _ "gitlab.com/gorib/waffle/config/tools/http"
	// _ "gitlab.com/gorib/waffle/config/service/sql"
)
`
	if err := os.WriteFile(filepath.Join("config", "di", "modules.go"), []byte(modules), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".env", []byte("SERVICE=demo\nDAEMON_PORT=9999\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var c Command
	if err := c.UncommentModule("gitlab.com/gorib/waffle/config/tools/http"); err != nil {
		t.Fatal(err)
	}
	if err := c.UncommentModule("gitlab.com/gorib/waffle/config/service/sql"); err != nil {
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
	if err := c.UncommentModule("gitlab.com/gorib/waffle/config/tools/http"); err != nil {
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
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	if err := os.MkdirAll(filepath.Join("config", "di"), 0755); err != nil {
		t.Fatal(err)
	}
	// Already active: there is nothing left to uncomment.
	modules := `package di

import (
	_ "gitlab.com/gorib/waffle/config/service/sql"
)
`
	if err := os.WriteFile(filepath.Join("config", "di", "modules.go"), []byte(modules), 0644); err != nil {
		t.Fatal(err)
	}

	var c Command
	if err := c.UncommentModule("gitlab.com/gorib/waffle/config/service/sql"); err != nil {
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
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	if err := os.MkdirAll(filepath.Join("config", "di"), 0755); err != nil {
		t.Fatal(err)
	}
	modules := `package di

import (
	// _ "gitlab.com/gorib/waffle/config/tools/http"
	// _ "gitlab.com/gorib/waffle/config/tools/grpc"
)
`
	if err := os.WriteFile(filepath.Join("config", "di", "modules.go"), []byte(modules), 0644); err != nil {
		t.Fatal(err)
	}

	var c Command
	for _, module := range []string{
		"gitlab.com/gorib/waffle/config/tools/http",
		"gitlab.com/gorib/waffle/config/tools/grpc",
	} {
		if err := c.UncommentModule(module); err != nil {
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

func assignments(env, key string) int {
	n := 0
	for _, line := range strings.Split(env, "\n") {
		if found, ok := assignment(line); ok && found == key {
			n++
		}
	}
	return n
}
