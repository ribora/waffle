package domain

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command/commandtest"
)

const (
	project = "example.com/svc"
	subject = "user" // the domain make-domain.md walks through
)

// A page that lists the files a command writes is a claim about the command, and
// nothing refused to build when it stopped being true: the page named
// service/user.go and a service.NewUserService long after the scaffolder wrote
// neither.
func TestMakeDomainPageListsWhatIsWritten(t *testing.T) {
	// Before the chdir: the page is found relative to the package, the files are
	// generated somewhere else entirely.
	documented := pathsIn(t, page(t))

	if err := Generate(commandtest.Project(t, project), subject, true); err != nil {
		t.Fatalf("generate: %v", err)
	}
	written := walk(t)

	for _, path := range documented {
		if !slices.Contains(written, path) {
			t.Errorf("make-domain.md documents %s, which the command does not write", path)
		}
	}
	for _, path := range written {
		if !slices.Contains(documented, path) {
			t.Errorf("the command writes %s, which make-domain.md does not mention", path)
		}
	}
}

// Every generated path, with the timestamp of a migration replaced: the docs
// cannot name a file whose name is the second it was created.
func walk(t *testing.T) []string {
	t.Helper()
	stamped := regexp.MustCompile(`\b\d{12,14}_`)

	var out []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || path == "go.mod" {
			return err
		}
		out = append(out, stamped.ReplaceAllString(filepath.ToSlash(path), "<timestamp>_"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

func page(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "docs", "cli", "make-domain.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// The page names a generated file as a bold path on a line of its own, which is
// what tells one apart from a path mentioned in passing.
var documentedPath = regexp.MustCompile("(?m)^\\*\\*`([^`]+)`\\*\\*")

func pathsIn(t *testing.T, body string) []string {
	t.Helper()
	var out []string
	for _, match := range documentedPath.FindAllStringSubmatch(body, -1) {
		out = append(out, match[1])
	}
	if len(out) == 0 {
		t.Fatal("no paths found in make-domain.md — the page changed shape, not the command")
	}
	slices.Sort(out)
	return out
}

// overview.md promises any subdirectory works; the tree used to be written below it.
func TestADomainScaffoldedFromASubdirectoryLandsAtTheRoot(t *testing.T) {
	root := commandtest.Project(t, project).Root()
	below := filepath.Join(root, "domain", "order", "service")
	if err := os.MkdirAll(below, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(below)
	p, err := command.OpenProject()
	if err != nil {
		t.Fatal(err)
	}
	if err := Generate(p, subject, true); err != nil {
		t.Fatal(err)
	}

	t.Chdir(root)
	written := walk(t)
	if !slices.Contains(written, "domain/user/service/service.go") {
		t.Errorf("the domain is not at the root:\n%s", strings.Join(written, "\n"))
	}
	for _, path := range written {
		if strings.HasPrefix(path, "domain/order/") {
			t.Errorf("%s was written below the working directory", path)
		}
	}
}

// The walk has to find the repository's own files, or the comparison above is
// between two empty sets.
func TestTheWalkSeesTheWholeDomain(t *testing.T) {
	if err := Generate(commandtest.Project(t, project), subject, true); err != nil {
		t.Fatal(err)
	}

	written := strings.Join(walk(t), "\n")
	for _, want := range []string{
		"domain/user/model/user.go",
		"domain/user/service/service.go",
		"domain/user/adapter/postgres/stored_user.go",
		"config/di/di_user_domain.go",
	} {
		if !strings.Contains(written, want) {
			t.Errorf("the walk missed %s:\n%s", want, written)
		}
	}
}
