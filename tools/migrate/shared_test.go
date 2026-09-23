package migrate

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func source(files map[string]string) fs.FS {
	out := fstest.MapFS{}
	for name, body := range files {
		out["migrations/"+name] = &fstest.MapFile{Data: []byte(body)}
	}
	return out
}

func TestCheckSharedAcceptsAVersionShippedTwiceVerbatim(t *testing.T) {
	err := CheckShared(map[string]fs.FS{
		"template":     source(map[string]string{"1_function.sql": "create function f()", "2_template.sql": "a"}),
		"notification": source(map[string]string{"1_function.sql": "create function f()", "3_notification.sql": "b"}),
	})

	if err != nil {
		t.Fatalf("identical copies were refused: %v", err)
	}
}

func TestCheckSharedRefusesAVersionShippedTwiceDifferently(t *testing.T) {
	err := CheckShared(map[string]fs.FS{
		"template":     source(map[string]string{"1_function.sql": "create function f()"}),
		"notification": source(map[string]string{"1_function.sql": "create function g()"}),
	})

	if err == nil {
		t.Fatal("two different migrations under one version were accepted")
	}
	for _, want := range []string{"1", "notification", "template"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %q: %v", want, err)
		}
	}
}

func TestCheckSharedComparesTheVersionNotTheName(t *testing.T) {
	err := CheckShared(map[string]fs.FS{
		"a": source(map[string]string{"1_create_function.sql": "x"}),
		"b": source(map[string]string{"1_other_name.sql": "y"}),
	})

	if err == nil {
		t.Fatal("goose keys on the version alone, and so must the check")
	}
}

func TestCheckSharedReportsASourceWithoutMigrations(t *testing.T) {
	err := CheckShared(map[string]fs.FS{"empty": fstest.MapFS{}})

	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("a source goose would fail on was accepted: %v", err)
	}
}
