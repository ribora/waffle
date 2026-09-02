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
var testTemplates embed.FS

func scaffoldIn(t *testing.T) (*Command, *bytes.Buffer) {
	t.Helper()

	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	var log bytes.Buffer
	output = &log
	t.Cleanup(func() { output = os.Stderr })

	return New("make:thing", "", testTemplates, nil), &log
}

// The whole contract is that your files are not touched, so the second run has to
// name what it did not touch. Silence there is indistinguishable from success.
func TestSecondRunSaysWhatItKept(t *testing.T) {
	c, log := scaffoldIn(t)
	sources := map[string]string{
		filepath.Join("testdata", "entity.go.tpl"): filepath.Join("domain", "user", "model", "user.go"),
	}
	params := map[string]any{"Domain": "user"}

	if err := c.Write(sources, params); err != nil {
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
	if err := c.Write(sources, params); err != nil {
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

// A target that cannot be read is the one case where skipping is wrong: it used to
// count as "already there" and pass in silence.
func TestUnreadableTargetIsAnError(t *testing.T) {
	c, _ := scaffoldIn(t)

	// A file where a directory has to go: Stat on the target fails with ENOTDIR,
	// which is neither "exists" nor "does not exist".
	if err := os.WriteFile("domain", []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}

	err := c.Write(map[string]string{
		filepath.Join("testdata", "entity.go.tpl"): filepath.Join("domain", "user", "model", "user.go"),
	}, map[string]any{"Domain": "user"})
	if err == nil {
		t.Fatal("scaffolding over a file that is not a directory reported success")
	}
	if !strings.Contains(err.Error(), "domain/user/model/user.go") {
		t.Errorf("the error does not name the target: %v", err)
	}
}
