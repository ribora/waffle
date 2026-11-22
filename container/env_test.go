package container

import (
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func keys(reads []EnvRead) []string {
	var out []string
	for _, read := range reads {
		out = append(out, read.Key)
	}
	return out
}

func TestEveryReaderIsFoundInBothGenericShapes(t *testing.T) {
	root := tree(t, map[string]string{"config/init.go": `package config

import "github.com/ribora/env"

func Init() {
	_ = env.Value("PLAIN", "")
	_ = env.NeedValue[string]("INSTANTIATED")
	_ = env.NeedArray[string, int]("LISTED")
	_ = env.Array[int]("ARRAY", nil)
	_, _ = env.MustJson[map[string]string]("MUST_JSON")
	_ = env.Load("NOT_A_READER")
	_ = env.Value(key, "")
	// _ = env.Value("COMMENTED", "")
}
`})

	got := map[string][2]string{}
	for _, read := range EnvReads(root) {
		typ := ""
		if read.Type != nil {
			typ = types.ExprString(read.Type)
		}
		got[read.Key] = [2]string{read.Reader, typ}
	}
	want := map[string][2]string{
		"PLAIN":        {"Value", ""},
		"INSTANTIATED": {"NeedValue", "string"},
		"LISTED":       {"NeedArray", ""},
		"ARRAY":        {"Array", "int"},
		"MUST_JSON":    {"MustJson", "map[string]string"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("reads %v, want %v", got, want)
	}
}

func TestTheImportIsFollowedUnderEveryAlias(t *testing.T) {
	root := tree(t, map[string]string{"config/init.go": `package config

import (
	environment "github.com/ribora/env"
	settings "github.com/ribora/env"
)

type fake struct{}

func (fake) Value(key string, fallback string) string { return fallback }

var env = fake{}

func Init() {
	_ = environment.Value("REAL", "")
	_ = settings.NeedValue[string]("ALSO_REAL")
	_ = env.Value("IMPOSTOR", "")
}
`})

	if got, want := keys(EnvReads(root)), []string{"REAL", "ALSO_REAL"}; !slices.Equal(got, want) {
		t.Errorf("keys %v, want %v", got, want)
	}
}

func TestOnlyTheServicesOwnSourceUnderTheRootIsRead(t *testing.T) {
	read := func(key string) string {
		return "package p\n\nimport \"github.com/ribora/env\"\n\nvar _ = env.NeedValue[string](\"" + key + "\")\n"
	}
	root := tree(t, map[string]string{
		"cmd/main.go":               read("CMD"),
		"domain/shop/shop.go":       read("DOMAIN"),
		"config/config.go":          read("CONFIG"),
		"config/diogen/di_gen.go":   read("GENERATED"),
		"domain/shop/shop_test.go":  read("TEST"),
		"tests/internal/harness.go": read("SUITE"),
		"pkg/client/client.go":      read("OUTSIDE"),
		"config/broken.go":          "package config\n\nfunc Init() { this is not go",
	})

	if got, want := keys(EnvReads(root)), []string{"CONFIG", "DOMAIN", "CMD"}; !slices.Equal(got, want) {
		t.Errorf("keys %v, want %v", got, want)
	}
}

func TestAReadCarriesItsCallSiteAndItsFilesImports(t *testing.T) {
	root := tree(t, map[string]string{"domain/shop/shop.go": "package shop\n\nimport \"github.com/ribora/env\"\n\n" +
		"var _ = env.NeedValue[string](\"SHOP_KEY\")\n"})

	reads := EnvReads(root)

	if len(reads) != 1 {
		t.Fatalf("reads %v, want one", keys(reads))
	}
	if got, want := reads[0].Pos.Filename, filepath.Join(root, "domain", "shop", "shop.go"); got != want {
		t.Errorf("file %s, want %s", got, want)
	}
	if got := reads[0].Pos.Line; got != 5 {
		t.Errorf("line %d, want 5", got)
	}
	if got := reads[0].Imports["env"]; got != "github.com/ribora/env" {
		t.Errorf("env imports %q", got)
	}
}
