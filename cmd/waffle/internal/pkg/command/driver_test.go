package command

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const scribusDrivers = "gitlab.com/gorib/waffle/config/tools/scribus/"

func project(t *testing.T, modules string) {
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

	if err := os.MkdirAll(filepath.Join("config", "di"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ModulesFile, []byte(modules), 0644); err != nil {
		t.Fatal(err)
	}
}

// A line to uncomment is all the precondition asks for: the first handler of a
// service leaves it commented, every one after it finds it already enabled.
func TestADriverWithALineIsAccepted(t *testing.T) {
	for _, line := range []string{
		`	// _ "gitlab.com/gorib/waffle/config/tools/scribus/amqp"`,
		`	_ "gitlab.com/gorib/waffle/config/tools/scribus/amqp"`,
	} {
		project(t, "package di\n\nimport (\n"+line+"\n)\n")

		if err := RequireDriver(scribusDrivers, "amqp"); err != nil {
			t.Errorf("requireDriver with %q: %v", strings.TrimSpace(line), err)
		}
	}
}

// The case that made this a precondition: nothing to uncomment leaves no trace,
// and the miss would surface at the end of the command as a diogen error.
func TestAModulesFileWithoutTheDriverLineFails(t *testing.T) {
	project(t, `package di

import (
	_ "gitlab.com/gorib/waffle/config/app"
	_ "gitlab.com/gorib/waffle/config/tools/scribus/single"
)
`)

	err := RequireDriver(scribusDrivers, "amqp")

	if err == nil {
		t.Fatal("a project with no driver line was accepted")
	}
	if !strings.Contains(err.Error(), "config/tools/scribus/amqp") {
		t.Errorf("the error does not name the line to add: %v", err)
	}
}

// The module path is built from it, so it may not carry a separator of its own.
func TestADriverThatIsNotAName(t *testing.T) {
	project(t, `package di

import (
	_ "gitlab.com/gorib/waffle/config/tools/scribus/amqp"
)
`)

	if err := RequireDriver(scribusDrivers, "../../service/amqp"); err == nil {
		t.Error("a path was accepted as a driver name")
	}
}

func TestTakeDriver(t *testing.T) {
	cases := []struct {
		name string
		args []string
		id   string
		rest []string
		fail bool
	}{
		{name: "separate value", args: []string{"--driver", "amqp", "user", "order_paid"}, id: "amqp", rest: []string{"user", "order_paid"}},
		{name: "joined value", args: []string{"--driver=amqp", "user", "order_paid"}, id: "amqp", rest: []string{"user", "order_paid"}},
		{name: "after the operands", args: []string{"user", "order_paid", "--driver", "amqp"}, id: "amqp", rest: []string{"user", "order_paid"}},
		{name: "no value", args: []string{"user", "order_paid", "--driver"}, fail: true},
		// Absent is not this function's refusal to make: Operands speaks first.
		{name: "absent", args: []string{"user", "order_paid"}, rest: []string{"user", "order_paid"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, rest, err := TakeDriver(c.args)
			if c.fail {
				if err == nil {
					t.Fatalf("TakeDriver(%v) = %q, want an error", c.args, id)
				}
				return
			}
			if err != nil {
				t.Fatalf("TakeDriver(%v): %v", c.args, err)
			}
			if id != c.id {
				t.Errorf("driver = %q, want %q", id, c.id)
			}
			if !reflect.DeepEqual(rest, c.rest) {
				t.Errorf("operands = %v, want %v", rest, c.rest)
			}
		})
	}
}
