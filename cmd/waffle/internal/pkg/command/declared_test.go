package command

import (
	"os"
	"path/filepath"
	"testing"
)

func pkg(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// The case that made this exist: warehouse declares Coordinates inside
// facility.go, so coordinates.go is absent and the name is taken.
func TestDeclaresFindsATypeInAnotherFile(t *testing.T) {
	dir := pkg(t, map[string]string{
		"facility.go": "package model\n\ntype Facility struct{}\n\ntype Coordinates struct{}\n",
	})

	where, found := Declares(dir, "Coordinates")
	if !found {
		t.Fatal("a type declared in a neighbouring file was reported as free")
	}
	if filepath.Base(where) != "facility.go" {
		t.Errorf("where = %s, want facility.go", where)
	}
}

func TestDeclaresIgnoresOtherNames(t *testing.T) {
	dir := pkg(t, map[string]string{
		"facility.go": "package model\n\ntype Facility struct{}\n",
	})

	if where, found := Declares(dir, "Unit"); found {
		t.Fatalf("Unit reported as declared in %s", where)
	}
}

// A name in a comment, a string or a field is not a declaration.
func TestDeclaresLooksAtDeclarationsOnly(t *testing.T) {
	dir := pkg(t, map[string]string{
		"facility.go": "package model\n\n// Coordinates belong to a Facility.\ntype Facility struct{ Coordinates string }\n",
	})

	if where, found := Declares(dir, "Coordinates"); found {
		t.Fatalf("a mention was taken for a declaration: %s", where)
	}
}

// Mid-scaffold the package routinely does not compile, and a directory that
// cannot be parsed must not be read as "the name is taken".
func TestDeclaresTreatsUnparsableFilesAsSilent(t *testing.T) {
	dir := pkg(t, map[string]string{
		"broken.go": "package model\n\ntype Coordinates struct{\n",
	})

	if where, found := Declares(dir, "Coordinates"); found {
		t.Fatalf("an unparsable file answered the question: %s", where)
	}
}

func TestDeclaresOnAMissingDirectory(t *testing.T) {
	if _, found := Declares(filepath.Join(t.TempDir(), "nope"), "Coordinates"); found {
		t.Fatal("a directory that is not there declared something")
	}
}
