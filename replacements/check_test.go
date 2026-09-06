package replacements

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seams are handed in rather than read off a container: what is under test here
// is the comparison, not the parsing.
func check(seams []Seam, found Inspection, cfg Config) error {
	root, err := ModuleRoot()
	if err != nil {
		return err
	}
	found.Seams = seams
	_, err = Verify(root, found, cfg)
	return err
}

const module = "example.test"

var seams = []Seam{
	{Field: "SqlDb", Types: []TypeRef{{Path: "gitlab.com/gorib/sql", Name: "Db"}}},
	{Field: "SecurityAdapter", Types: []TypeRef{{Path: module + "/domain/service", Name: "SecurityAdapter"}}},
	{Field: "Merged", Types: []TypeRef{
		{Path: module + "/domain/a", Name: "Management"},
		{Path: module + "/domain/b", Name: "Management"},
	}},
}

func allow(entries map[string][]string) Config {
	return Config{Allow: entries}
}

const literal = `package feature

import "example.test/config/diogen"

func boot() {
	_, _ = diogen.InitAppWith(diogen.Replacements{
		SecurityAdapter: nil,
	})
}
`

func TestRefusesAFieldTheListDoesNotGrant(t *testing.T) {
	write(t, map[string]string{"tests/feature/steps_test.go": literal})

	err := check(seams, containers(), allow(nil))
	if err == nil {
		t.Fatal("check passed a field that replacements.allow does not name")
	}
	for _, want := range []string{
		"SecurityAdapter",
		"example.test/domain/service.SecurityAdapter",
		"tests/feature/steps_test.go:7",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%s", want, err)
		}
	}
}

// Named by import path and interface, not by field: the field name is diogen's
// and changes when registrations do.
func TestPassesAGrantedField(t *testing.T) {
	write(t, map[string]string{"tests/feature/steps_test.go": literal})

	cfg := allow(map[string][]string{module + "/domain/service": {"SecurityAdapter"}})
	if err := check(seams, containers(), cfg); err != nil {
		t.Fatalf("check refused a granted field: %v", err)
	}
}

func TestReadsTheFieldByFieldForm(t *testing.T) {
	write(t, map[string]string{"tests/feature/steps_test.go": `package feature

import "example.test/config/diogen"

func boot() {
	var cut diogen.Replacements
	cut.SqlDb = nil
}
`})

	err := check(seams, containers(), allow(nil))
	if err == nil {
		t.Fatal("a field assigned one by one was not read as a replacement")
	}
	if !strings.Contains(err.Error(), "SqlDb") {
		t.Errorf("the refusal does not name SqlDb:\n%s", err)
	}
}

func TestIgnoresTheHarness(t *testing.T) {
	write(t, map[string]string{"tests/internal/harness/harness.go": `package harness

import "example.test/config/diogen"

func Boot(cut diogen.Replacements) {
	cut.SqlDb = nil
}
`})

	if err := check(seams, containers(), allow(nil)); err != nil {
		t.Fatalf("check read the harness: %v", err)
	}
}

func TestIgnoresTheGeneratedContainerOnly(t *testing.T) {
	root := write(t, map[string]string{
		"config/diogen/di_gen.go": `package diogen

type Replacements struct{}

func InitAppWith(r Replacements) {
	_ = Replacements{SqlDb: nil}
}
`,
		"config/diogen/app_test.go": literal,
	})

	found, _, err := scan(root, Inspection{
		Paths: []string{module + "/config/diogen"},
		Files: []string{filepath.Join(root, "config", "diogen", "di_gen.go")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Field != "SecurityAdapter" {
		t.Fatalf("scan found %v, want the one field the test replaces", found)
	}
}

func TestRefusesAPartlyGrantedComposite(t *testing.T) {
	write(t, map[string]string{"tests/feature/steps_test.go": `package feature

import "example.test/config/diogen"

func boot() {
	_, _ = diogen.InitAppWith(diogen.Replacements{Merged: nil})
}
`})

	cfg := allow(map[string][]string{module + "/domain/a": {"Management"}})
	err := check(seams, containers(), cfg)
	if err == nil {
		t.Fatal("check passed a composite with an ungranted constituent")
	}
	if !strings.Contains(err.Error(), "domain/b.Management") {
		t.Errorf("the refusal does not name the ungranted constituent:\n%s", err)
	}
	if strings.Contains(err.Error(), "domain/a.Management") {
		t.Errorf("the refusal names the granted constituent:\n%s", err)
	}
}

// Fatal, not a warning: the container is older than the test, so every other
// answer the check gives is guesswork.
func TestFailsOnAFieldNoContainerExposes(t *testing.T) {
	write(t, map[string]string{"tests/feature/steps_test.go": `package feature

import "example.test/config/diogen"

func boot() {
	_, _ = diogen.InitAppWith(diogen.Replacements{Gone: nil})
}
`})

	err := check(seams, containers(), allow(nil))
	if err == nil || !strings.Contains(err.Error(), "no field Gone") {
		t.Fatalf("err = %v, want a failure naming the unknown field", err)
	}
}

func TestRefusesALiteralWithoutFieldNames(t *testing.T) {
	write(t, map[string]string{"tests/feature/steps_test.go": `package feature

import "example.test/config/diogen"

func boot() {
	_, _ = diogen.InitAppWith(diogen.Replacements{nil, nil, nil})
}
`})

	err := check(seams, containers(), allow(nil))
	if err == nil || !strings.Contains(err.Error(), "without field names") {
		t.Fatalf("err = %v, want a failure naming the unkeyed literal", err)
	}
}

func TestIgnoresAnUnrelatedReplacementsStruct(t *testing.T) {
	write(t, map[string]string{"tests/feature/steps_test.go": `package feature

import "example.test/pkg/other"

func boot() {
	_ = other.Replacements{SqlDb: nil}
}
`})

	if err := check(seams, containers(), allow(nil)); err != nil {
		t.Fatalf("check read a struct from another package: %v", err)
	}
}

// It has to survive the passing check, or nobody would ever see it.
func TestWarnsAboutAGrantNoContainerExposes(t *testing.T) {
	root := write(t, map[string]string{"tests/feature/steps_test.go": literal})

	cfg := allow(map[string][]string{module + "/domain/service": {"SecurityAdapter", "Renamed"}})
	found := containers()
	found.Seams = seams
	rep, err := Verify(root, found, cfg)
	if err != nil {
		t.Fatalf("check refused a granted field: %v", err)
	}
	if len(rep.Warnings) != 1 || !strings.Contains(rep.Warnings[0], module+"/domain/service.Renamed") {
		t.Fatalf("warnings = %v, want the one entry nothing exposes", rep.Warnings)
	}
}

func containers() Inspection {
	return Inspection{Paths: []string{module + "/config/diogen"}}
}

// Chdir, because the check walks up to go.mod from the working directory.
func write(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module " + module + "\n\ngo 1.27\n"
	for name, body := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	return root
}
