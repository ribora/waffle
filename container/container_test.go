package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func inTempDir(t *testing.T) {
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
}

func writeGo(t *testing.T, path, body string) {
	t.Helper()
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverContainers(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  []Container
	}{
		{
			name: "scaffolded directive",
			files: map[string]string{
				filepath.Join("config", "di", "di.go"): "//go:generate go tool diogen -output config/diogen/di_gen.go $GOFILE\n\npackage di\n",
			},
			want: []Container{{File: "config/diogen/di_gen.go", Dir: "config/diogen", Package: "diogen"}},
		},
		{
			name: "equals form",
			files: map[string]string{
				filepath.Join("config", "di", "di.go"): "//go:generate diogen -output=gen/app.go -pkg=gen $GOFILE\n\npackage di\n",
			},
			want: []Container{{File: "gen/app.go", Dir: "gen", Package: "gen"}},
		},
		{
			name: "pkg omitted falls back to the output directory",
			files: map[string]string{
				filepath.Join("config", "di", "di.go"): "//go:generate diogen -output config/diogen/di_gen.go $GOFILE\n\npackage di\n",
			},
			want: []Container{{File: "config/diogen/di_gen.go", Dir: "config/diogen", Package: "diogen"}},
		},
		{
			name: "two containers, sorted",
			files: map[string]string{
				filepath.Join("worker", "di", "di.go"): "//go:generate diogen -output worker/diogen/di_gen.go $GOFILE\n\npackage di\n",
				filepath.Join("api", "di", "di.go"):    "//go:generate diogen -output api/diogen/di_gen.go $GOFILE\n\npackage di\n",
			},
			want: []Container{
				{File: "api/diogen/di_gen.go", Dir: "api/diogen", Package: "diogen"},
				{File: "worker/diogen/di_gen.go", Dir: "worker/diogen", Package: "diogen"},
			},
		},
		{
			name: "other generators ignored",
			files: map[string]string{
				"store.go":                             "//go:generate go tool store -output stored.go $GOFILE\n\npackage x\n",
				filepath.Join("config", "di", "di.go"): "//go:generate diogen -output config/diogen/di_gen.go $GOFILE\n\npackage di\n",
			},
			want: []Container{{File: "config/diogen/di_gen.go", Dir: "config/diogen", Package: "diogen"}},
		},
		{
			name: "testdata and vendor skipped",
			files: map[string]string{
				filepath.Join("testdata", "cases", "x", "setup.go"): "//go:generate diogen -output testdata/cases/x/di_gen.go -pkg x $GOFILE\n\npackage x\n",
				filepath.Join("vendor", "dep", "di.go"):             "//go:generate diogen -output vendor/dep/di_gen.go -pkg dep $GOFILE\n\npackage dep\n",
				filepath.Join("config", "di", "di.go"):              "//go:generate diogen -output config/diogen/di_gen.go $GOFILE\n\npackage di\n",
			},
			want: []Container{{File: "config/diogen/di_gen.go", Dir: "config/diogen", Package: "diogen"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inTempDir(t)
			for path, body := range tc.files {
				writeGo(t, path, body)
			}

			got, err := DiscoverContainers(".")
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("found %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestDiscoverContainersWithoutDirective(t *testing.T) {
	inTempDir(t)
	writeGo(t, filepath.Join("config", "di", "di.go"), "package di\n")

	_, err := DiscoverContainers(".")
	if err == nil {
		t.Fatal("a tree with no diogen directive discovered a container")
	}
	if !strings.Contains(err.Error(), "diogen") {
		t.Errorf("the error does not name what was looked for: %v", err)
	}
}
