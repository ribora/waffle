package command

import (
	"bytes"
	"embed"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//go:embed testdata/*
var testFiles embed.FS

var testTemplates = NewTemplates(testFiles)

func scaffoldIn(t *testing.T) (*Project, *bytes.Buffer) {
	t.Helper()

	dir := t.TempDir()
	t.Chdir(dir)

	var log bytes.Buffer
	output = &log
	t.Cleanup(func() { output = os.Stderr })

	return &Project{root: dir}, &log
}

// The whole contract is that your files are not touched, so the second run has to
// name what it did not touch. Silence there is indistinguishable from success.
func TestSecondRunSaysWhatItKept(t *testing.T) {
	p, log := scaffoldIn(t)
	sources := map[string]string{
		filepath.Join("testdata", "entity.go.tpl"): filepath.Join("domain", "user", "model", "user.go"),
	}
	params := map[string]any{"Domain": "user"}

	if err := p.Write(testTemplates, sources, params); err != nil {
		t.Fatal(err)
	}
	if got := log.String(); !strings.Contains(got, "+ domain/user/model/user.go") {
		t.Errorf("the created file was not reported:\n%s", got)
	}

	// A hand edit is what the protection exists for: it has to survive.
	target := filepath.Join("domain", "user", "model", "user.go")
	mine := "package model\n\n// mine\n"
	if err := os.WriteFile(target, []byte(mine), 0644); err != nil {
		t.Fatal(err)
	}

	log.Reset()
	if err := p.Write(testTemplates, sources, params); err != nil {
		t.Fatal(err)
	}
	got := log.String()
	if !strings.Contains(got, "domain/user/model/user.go") || !strings.Contains(got, "exists") {
		t.Errorf("the kept file was not reported:\n%s", got)
	}
	if strings.Contains(got, "+ domain/user/model/user.go") {
		t.Errorf("an existing file was reported as created:\n%s", got)
	}

	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != mine {
		t.Errorf("the hand-edited file was overwritten:\n%s", string(after))
	}
}

func TestKeptDeclarationIsMarkedKept(t *testing.T) {
	_, log := scaffoldIn(t)

	KeptAs("model.User", filepath.Join("domain", "user", "model", "user.go"))

	got := log.String()
	if !strings.Contains(got, "= model.User — declared in domain/user/model/user.go, kept") {
		t.Errorf("the kept declaration was not marked as kept:\n%s", got)
	}
	if strings.Contains(got, "+ model.User") {
		t.Errorf("a kept declaration was reported as created:\n%s", got)
	}
}

// A target that cannot be read is the one case where skipping is wrong: it used to
// count as "already there" and pass in silence.
func TestUnreadableTargetIsAnError(t *testing.T) {
	p, _ := scaffoldIn(t)

	// A file where a directory has to go: Stat on the target fails with ENOTDIR,
	// which is neither "exists" nor "does not exist".
	if err := os.WriteFile("domain", []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}

	err := p.Write(testTemplates, map[string]string{
		filepath.Join("testdata", "entity.go.tpl"): filepath.Join("domain", "user", "model", "user.go"),
	}, map[string]any{"Domain": "user"})
	if err == nil {
		t.Fatal("scaffolding over a file that is not a directory reported success")
	}
	if !strings.Contains(err.Error(), "domain/user/model/user.go") {
		t.Errorf("the error does not name the target: %v", err)
	}
}

func TestAProjectOpenedBelowTheRootWritesAtTheRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test\n")
	if err := os.MkdirAll(filepath.Join(root, "domain", "user"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(root, "domain", "user"))
	p, err := OpenProject()
	if err != nil {
		t.Fatal(err)
	}

	var log bytes.Buffer
	output = &log
	t.Cleanup(func() { output = os.Stderr })
	err = p.Write(testTemplates, map[string]string{
		filepath.Join("testdata", "entity.go.tpl"): filepath.Join("domain", "user", "model", "user.go"),
	}, map[string]any{"Domain": "user"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "domain", "user", "model", "user.go")); err != nil {
		t.Errorf("the file is not under the root: %v", err)
	}
	if _, err := os.Stat("domain"); err == nil {
		t.Error("a nested domain/ was written below the working directory")
	}
	if got := log.String(); !strings.Contains(got, "+ domain/user/model/user.go\n") {
		t.Errorf("the report does not name the root-relative path:\n%s", got)
	}
}

func TestWriteIfChanged(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nested", "file.yaml")
	for i, want := range []bool{true, false} {
		changed, err := WriteIfChanged(target, []byte("body\n"))
		if err != nil {
			t.Fatal(err)
		}
		if changed != want {
			t.Errorf("run %d: changed = %v, want %v", i+1, changed, want)
		}
	}
	if changed, _ := WriteIfChanged(target, []byte("other\n")); !changed {
		t.Error("a different body was not written")
	}
}
