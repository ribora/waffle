package gomod

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRequires(t *testing.T) {
	for _, c := range []struct {
		name string
		mod  string
		want string
	}{
		{
			name: "in a require block",
			mod: `module svc

require (
	gitlab.com/gorib/http v1.12.0
	gitlab.com/gorib/waffle v1.11.2
)
`,
			want: "v1.11.2",
		},
		{
			name: "on its own line",
			mod:  "module svc\n\nrequire gitlab.com/gorib/waffle v1.11.2\n",
			want: "v1.11.2",
		},
		{
			// A path on one machine is not a version anyone else can resolve.
			name: "replaced",
			mod: `module svc

require gitlab.com/gorib/waffle v1.11.2

replace gitlab.com/gorib/waffle => ../waffle
`,
			want: "",
		},
		{
			name: "absent",
			mod:  "module svc\n\nrequire gitlab.com/gorib/http v1.12.0\n",
			want: "",
		},
		{
			name: "unparseable",
			mod:  "this is not a go.mod",
			want: "",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(c.mod), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := Requires(dir, "gitlab.com/gorib/waffle"); got != c.want {
				t.Errorf("version = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRequiresWithoutAGoMod(t *testing.T) {
	if got := Requires(t.TempDir(), "gitlab.com/gorib/waffle"); got != "" {
		t.Errorf("version = %q, want empty", got)
	}
}
