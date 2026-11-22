package domain

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	httpcmd "github.com/ribora/waffle/cmd/waffle/internal/commands/http"
	"github.com/ribora/waffle/cmd/waffle/internal/commands/service"
)

// The path init-service.md documents, through the real generators: a domain
// alone is a container diogen refuses, and its first transport completes it.
func TestADomainJoinsTheContainerWithItsFirstTransport(t *testing.T) {
	if testing.Short() {
		t.Skip("installs diogen, store and whisk, and builds the service")
	}
	checkout, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWORK", "off")
	t.Chdir(t.TempDir())

	run(t, "go", "mod", "init", "catalog")
	run(t, "go", "mod", "edit", "-replace", "github.com/ribora/waffle="+checkout)
	if err := service.New().Run(t.Context()); err != nil {
		t.Fatalf("init:service: %v", err)
	}
	if err := New().Run(t.Context(), subject); err != nil {
		t.Fatalf("make:domain: %v", err)
	}

	out, err := exec.Command("go", "generate", "./...").CombinedOutput()
	if err == nil || !strings.Contains(string(out), "catalog/domain/user/service.NewUserManagement") {
		t.Fatalf("before a transport, diogen should refuse the unread management, got %v:\n%s", err, out)
	}

	if err := httpcmd.New().Run(t.Context(), subject); err != nil {
		t.Fatalf("make:domain:http: %v", err)
	}
	run(t, "go", "build", "./...")
}

func run(t *testing.T, name string, args ...string) {
	t.Helper()
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}
