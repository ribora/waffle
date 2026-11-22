package scribus

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"text/template"

	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command"
	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command/commandtest"
	"github.com/ribora/waffle/tools/scribus"
)

// Nothing compiles these templates, so a handler that stops satisfying its
// interface stays invisible until a scaffolded service fails to build.

func render(t *testing.T, mode, subject string) string {
	t.Helper()

	body, err := templateFiles.ReadFile(filepath.Join("templates", mode, "handler.go.tpl"))
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := template.New("handler").Funcs(command.Funcs()).Parse(string(body))
	if err != nil {
		t.Fatalf("parse %s template: %v", mode, err)
	}

	var out bytes.Buffer
	// ValidateName refuses orderPaid: the fixture must render what can be entered.
	err = tpl.Execute(&out, map[string]string{
		"Domain":  "user",
		"Event":   "order_paid",
		"Subject": subject,
		"Project": "example.com/svc",
	})
	if err != nil {
		t.Fatalf("render %s template: %v", mode, err)
	}
	return out.String()
}

func methodsOf(t *testing.T, src string) map[string]bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "handler.go", src, 0)
	if err != nil {
		t.Fatalf("the rendered handler does not parse: %v\n%s", err, src)
	}
	out := map[string]bool{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil {
			out[fn.Name.Name] = true
		}
	}
	return out
}

func TestHandlerTemplatesSatisfyTheirInterfaces(t *testing.T) {
	cases := []struct {
		mode  string
		iface reflect.Type
	}{
		{"single", reflect.TypeFor[scribus.Handler]()},
		{"batch", reflect.TypeFor[scribus.BatchHandler]()},
	}
	for _, c := range cases {
		t.Run(c.mode, func(t *testing.T) {
			declared := methodsOf(t, render(t, c.mode, "order.paid"))
			for i := range c.iface.NumMethod() {
				name := c.iface.Method(i).Name
				if !declared[name] {
					t.Errorf("the %s template declares no %s, so the scaffolded handler is not a %s",
						c.mode, name, c.iface.Name())
				}
			}
		})
	}
}

func renderFile(t *testing.T, path, event string) string {
	t.Helper()
	body, err := templateFiles.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := template.New(filepath.Base(path)).Funcs(command.Funcs()).Parse(string(body))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out bytes.Buffer
	if err := tpl.Execute(&out, map[string]string{"Domain": "user", "Event": event, "Subject": event, "Project": "example.com/svc"}); err != nil {
		t.Fatalf("render %s: %v", path, err)
	}
	return out.String()
}

func TestTwoHandlersOfADomainShareOneManagement(t *testing.T) {
	for _, mode := range []string{"single", "batch"} {
		t.Run(mode, func(t *testing.T) {
			pkg := []string{
				renderFile(t, filepath.Join("templates", mode, "handler.go.tpl"), "order_paid"),
				renderFile(t, filepath.Join("templates", mode, "handler.go.tpl"), "order_cancelled"),
				renderFile(t, filepath.Join("templates", "management.go.tpl"), "order_paid"),
			}
			declared := 0
			for _, src := range pkg {
				declared += strings.Count(src, "type UserManagement interface")
			}
			if declared != 1 {
				t.Errorf("UserManagement is declared %d times in the handlers' package, want once", declared)
			}

			wired := 0
			for _, src := range []string{
				renderFile(t, filepath.Join("templates", mode, "di.go.tpl"), "order_paid"),
				renderFile(t, filepath.Join("templates", mode, "di.go.tpl"), "order_cancelled"),
				renderFile(t, filepath.Join("templates", "di.go.tpl"), "order_paid"),
			} {
				wired += strings.Count(src, "di.Wire[scribusUser.UserManagement]")
			}
			if wired != 1 {
				t.Errorf("the management is wired %d times, want once", wired)
			}
		})
	}
}

// Each case is a service whose binary would stop at startup, so each is refused
// before the first file is written.
func TestASecondConsumerShapeIsRefused(t *testing.T) {
	cases := []struct {
		name    string
		modules string
		di      string
		args    []string
		names   string
	}{
		{
			name:    "batch beside an active single",
			modules: "\t// _ \"" + driverPrefix + "amqp\"\n\t_ \"" + driverPrefix + "single\"\n\t// _ \"" + driverPrefix + "batch\"\n",
			args:    []string{"--driver", "amqp", "--batch", "user", "order_paid"},
			names:   driverPrefix + "single",
		},
		{
			name:    "single beside an active batch",
			modules: "\t// _ \"" + driverPrefix + "amqp\"\n\t// _ \"" + driverPrefix + "single\"\n\t_ \"" + driverPrefix + "batch\"\n",
			args:    []string{"--driver", "amqp", "user", "order_paid"},
			names:   driverPrefix + "batch",
		},
		{
			name:    "a second driver",
			modules: "\t// _ \"" + driverPrefix + "amqp\"\n\t_ \"" + driverPrefix + "nats\"\n\t// _ \"" + driverPrefix + "single\"\n",
			args:    []string{"--driver", "amqp", "user", "order_paid"},
			names:   driverPrefix + "nats",
		},
		{
			name:    "a mode given as the driver",
			modules: "\t// _ \"" + driverPrefix + "single\"\n\t// _ \"" + driverPrefix + "batch\"\n",
			args:    []string{"--driver", "batch", "user", "order_paid"},
			names:   "batch is a mode",
		},
		{
			name:    "a second BatchHandler",
			modules: "\t_ \"" + driverPrefix + "amqp\"\n\t_ \"" + driverPrefix + "batch\"\n",
			di:      "package di\n\nfunc InitDi() {\n\tdi.Wire[wscribus.BatchHandler](scribusUser.NewOrderPaidCommand)\n}\n",
			args:    []string{"--driver", "amqp", "--batch", "user", "order_cancelled"},
			names:   "config/di/di.go",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := commandtest.Project(t, "example.com/svc")
			files := map[string]string{
				command.ModulesFile:         "package di\n\nimport (\n" + c.modules + ")\n",
				"domain/user/model/user.go": "package model\n",
			}
			if c.di != "" {
				files["config/di/di.go"] = c.di
			}
			for name, body := range files {
				if err := os.MkdirAll(filepath.Dir(p.Path(name)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p.Path(name), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			err := New().Run(t.Context(), c.args...)

			if err == nil || !strings.Contains(err.Error(), c.names) {
				t.Fatalf("err = %v, want a refusal naming %s", err, c.names)
			}
			if _, err := os.Stat(p.Path("domain", "user", "cmd")); err == nil {
				t.Error("the refusal came after the handler was written")
			}
		})
	}
}

// Both templates must bind the subject, not the event that named the type.
func TestKeysReturnTheSubject(t *testing.T) {
	for _, mode := range []string{"single", "batch"} {
		src := render(t, mode, "order.paid")
		if !strings.Contains(src, `[]string{"order.paid"}`) {
			t.Errorf("%s template does not bind the subject:\n%s", mode, src)
		}
		if strings.Contains(src, `[]string{"order_paid"}`) {
			t.Errorf("%s template still binds the event name as the subject", mode)
		}
	}
}
