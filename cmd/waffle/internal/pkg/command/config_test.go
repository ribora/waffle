package command

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// A key waffle does not read is a file that does not say what its author meant:
// the list of replacements and the areas are not waffle's keys.
func TestProjectConfigRejectsAKeyItDoesNotRead(t *testing.T) {
	cases := map[string]string{
		"unknown key":  "mocks:\n  generate:\n    gitlab.com/gorib/sql: [Db]\n",
		"replacements": "replacements:\n  allow:\n    gitlab.com/gorib/sql: [Db]\n",
		"agents":       "agents:\n  areas:\n    events:\n      paths: [../events/]\n",
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "go.mod", "module example.test\n\ngo 1.27\n")
			writeFile(t, root, "waffle.yaml", body)
			t.Chdir(root)

			if _, err := project(t).Config(); err == nil {
				t.Fatal("the file loaded")
			} else if !strings.Contains(err.Error(), "waffle.yaml") {
				t.Errorf("err = %v, want a message naming the file", err)
			}
		})
	}
}

func TestAMalformedWaffleYamlStopsOnlyWhatReadsIt(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test\n\ngo 1.27\n")
	writeFile(t, root, "waffle.yaml", "mapper: [\n")
	t.Chdir(root)

	p, err := OpenProject()
	if err != nil {
		t.Fatalf("the project did not open: %v", err)
	}
	if _, err := p.Config(); err == nil {
		t.Fatal("the config loaded")
	}
}

func TestAWaffleYamlWithNothingInItIsTheDefaults(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test\n\ngo 1.27\n")
	t.Chdir(root)
	defaults, err := project(t).Config()
	if err != nil {
		t.Fatal(err)
	}

	for name, body := range map[string]string{
		"empty":         "",
		"comments only": "# Project conventions for waffle generators.\n\n# mapper: dto\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got := configFrom(t, body); !reflect.DeepEqual(got, defaults) {
				t.Errorf("config = %+v, want the defaults %+v", got, defaults)
			}
		})
	}
}

func configFrom(t *testing.T, body string) Config {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test\n\ngo 1.27\n")
	writeFile(t, root, "waffle.yaml", body)
	t.Chdir(root)

	cfg, err := project(t).Config()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func project(t *testing.T) *Project {
	t.Helper()
	p, err := OpenProject()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func writeFile(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
