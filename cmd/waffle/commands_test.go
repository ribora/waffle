package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// An empty directory on purpose: a command that lets the extra word through
// reaches for go.mod and fails with something else, which must not read as a pass.
func TestEveryCommandRefusesAnExtraOperand(t *testing.T) {
	t.Chdir(t.TempDir())
	cmds := newCommands()

	for code, cmd := range cmds.All() {
		err := cmd.Run(t.Context(), "a", "b", "c", "d")
		if err == nil {
			t.Errorf("waffle %s accepted four operands", code)
			continue
		}
		// Not `"d"`: each command names its own first extra operand.
		if !strings.Contains(err.Error(), "unexpected operand") {
			t.Errorf("waffle %s: %v\nwant a refusal naming the operand with no slot", code, err)
		}
	}
}

const modules = `package di

import (
	// _ "gitlab.com/gorib/waffle/config/service/sql"
	// _ "gitlab.com/gorib/waffle/config/tools/migrate"
	// _ "gitlab.com/gorib/waffle/config/tools/http"
	// _ "gitlab.com/gorib/waffle/config/tools/grpc"
	// _ "gitlab.com/gorib/waffle/config/tools/scribus/amqp"
	// _ "gitlab.com/gorib/waffle/config/tools/scribus/single"
)
`

// Every module line a command could enable, and no domain.
func withoutDomains(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	for name, body := range map[string]string{"go.mod": "module example.com/svc\n", "config/di/modules.go": modules} {
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func files(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			out = append(out, filepath.ToSlash(path))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEveryCommandAddingToADomainRefusesOneThatIsNotThere(t *testing.T) {
	cmds := newCommands()
	for _, args := range [][]string{
		{"make:domain:http", "user"},
		{"make:domain:grpc", "user"},
		{"make:domain:scribus", "--driver", "amqp", "user", "order_paid"},
		{"make:domain:command", "user", "recalc"},
		{"make:domain:entity", "user", "address"},
		{"make:domain:migrate", "user"},
		{"make:domain:migration", "user", "add_status"},
	} {
		t.Run(args[0], func(t *testing.T) {
			withoutDomains(t)
			before := files(t)

			err := cmds.Get(args[0]).Run(t.Context(), args[1:]...)

			if err == nil || !strings.Contains(err.Error(), "domain/user does not exist") {
				t.Fatalf("err = %v, want a refusal naming domain/user", err)
			}
			if after := files(t); !slices.Equal(after, before) {
				t.Errorf("the refusal came after a write: %v", after)
			}
		})
	}
}

func TestAMigrationIsRefusedWhereNoRunnerAppliesIt(t *testing.T) {
	withoutDomains(t)
	if err := os.MkdirAll(filepath.Join("domain", "user", "model"), 0o755); err != nil {
		t.Fatal(err)
	}
	before := files(t)
	cmds := newCommands()

	err := cmds.Get("make:domain:migration").Run(t.Context(), "user", "add_status")

	if err == nil || !strings.Contains(err.Error(), "domain/user/cmd/migrate/main.go") {
		t.Fatalf("err = %v, want a refusal naming the runner", err)
	}
	if after := files(t); !slices.Equal(after, before) {
		t.Errorf("the refusal came after a write: %v", after)
	}
}

func TestEveryCommandRefusesAnUnknownFlag(t *testing.T) {
	t.Chdir(t.TempDir())
	cmds := newCommands()

	for code, cmd := range cmds.All() {
		err := cmd.Run(t.Context(), "--not-a-flag")
		if err == nil {
			t.Errorf("waffle %s accepted an unknown flag", code)
			continue
		}
		if !strings.Contains(err.Error(), `unknown flag "--not-a-flag"`) {
			t.Errorf("waffle %s: %v\nwant a refusal naming the flag", code, err)
		}
	}
}
