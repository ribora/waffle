package scribus

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"text/template"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	"gitlab.com/gorib/waffle/tools/scribus"
)

// Nothing compiles these templates, so a handler that stops satisfying its
// interface stays invisible until a scaffolded service fails to build.

func render(t *testing.T, mode, key string) string {
	t.Helper()

	body, err := templates.ReadFile(filepath.Join("templates", mode, "handler.go.tpl"))
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
		"Key":     key,
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

// Both templates must bind the key, not the event that named the type.
func TestKeysReturnTheRoutingKey(t *testing.T) {
	for _, mode := range []string{"single", "batch"} {
		src := render(t, mode, "order.paid")
		if !strings.Contains(src, `[]string{"order.paid"}`) {
			t.Errorf("%s template does not bind the routing key:\n%s", mode, src)
		}
		if strings.Contains(src, `[]string{"order_paid"}`) {
			t.Errorf("%s template still binds the event name as the key", mode)
		}
	}
}
