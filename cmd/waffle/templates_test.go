package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"text/template"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/plural"
)

// Nothing compiles or parses the scaffolder's templates, so a broken one is
// found by whoever runs the command next. Parameter names come from the
// command's source, not from the template: read off the template, a typo would
// define the very field it misspells.

var (
	// {{ .X }}, {{ title .X }}, {{- if .X }}
	tplField  = regexp.MustCompile(`\{\{-?\s*(?:\w+\s+)?\.(\w+)`)
	tplToggle = regexp.MustCompile(`\{\{-?\s*if\s+\.(\w+)`)
)

func commandDirs(t *testing.T) []string {
	t.Helper()
	root := filepath.Join(repoRoot(t), "cmd", "waffle", "internal", "commands")

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if len(templatesIn(t, dir)) > 0 {
			out = append(out, dir)
		}
	}
	if len(out) == 0 {
		t.Fatal("no command templates found — the layout moved")
	}
	return out
}

func templatesIn(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go.tpl") {
			return err
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Every map[string]string / map[string]any literal in the package, which is how
// a command hands its parameters to Write.
func suppliedParams(t *testing.T, dir string) map[string]bool {
	t.Helper()

	out := map[string]bool{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || !isParamMap(lit.Type) {
				return true
			}
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if key, ok := kv.Key.(*ast.BasicLit); ok && key.Kind == token.STRING {
					out[strings.Trim(key.Value, `"`)] = true
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", dir, err)
	}
	return out
}

func isParamMap(expr ast.Expr) bool {
	m, ok := expr.(*ast.MapType)
	if !ok {
		return false
	}
	if key, ok := m.Key.(*ast.Ident); !ok || key.Name != "string" {
		return false
	}
	val, ok := m.Value.(*ast.Ident)
	return ok && (val.Name == "string" || val.Name == "any")
}

func referencedFields(body string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range tplField.FindAllStringSubmatch(body, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

func values(fields []string, body string, toggles bool) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		if f == "Project" {
			out[f] = "example.com/svc"
			continue
		}
		out[f] = "thing"
	}
	for _, m := range tplToggle.FindAllStringSubmatch(body, -1) {
		out[m[1]] = toggles
	}
	return out
}

func funcs() template.FuncMap {
	return template.FuncMap{
		"plural": plural.Plural,
		"title":  func(w string) string { return strings.ToUpper(w[:1]) + w[1:] },
	}
}

func TestTemplatesUseOnlySuppliedParams(t *testing.T) {
	root := repoRoot(t)
	for _, dir := range commandDirs(t) {
		supplied := suppliedParams(t, dir)
		for _, path := range templatesIn(t, dir) {
			rel, _ := filepath.Rel(root, path)
			t.Run(rel, func(t *testing.T) {
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				for _, field := range referencedFields(string(body)) {
					if !supplied[field] {
						t.Errorf("references .%s, which the command never supplies (it passes: %s)",
							field, strings.Join(sortedKeys(supplied), ", "))
					}
				}
			})
		}
	}
}

func TestTemplatesRenderToGo(t *testing.T) {
	root := repoRoot(t)
	for _, dir := range commandDirs(t) {
		for _, path := range templatesIn(t, dir) {
			rel, _ := filepath.Rel(root, path)
			t.Run(rel, func(t *testing.T) {
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				tpl, err := template.New(filepath.Base(path)).Funcs(funcs()).Parse(string(body))
				if err != nil {
					t.Fatalf("template does not parse: %v", err)
				}

				fields := referencedFields(string(body))
				// Both branches of every conditional: the off path is the one
				// nobody looks at.
				for _, toggles := range []bool{true, false} {
					var out bytes.Buffer
					if err := tpl.Execute(&out, values(fields, string(body), toggles)); err != nil {
						t.Fatalf("toggles=%v: render: %v", toggles, err)
					}
					src := out.String()
					if _, err := parser.ParseFile(token.NewFileSet(), "rendered.go", src, 0); err != nil {
						t.Errorf("toggles=%v: rendered output is not Go: %v\n%s", toggles, err, src)
					}
				}
			})
		}
	}
}

// The rule is in docs/core/commands.md; the compiler cannot check it, because
// the global getopt functions have the same signatures as the local ones.
func TestCommandTemplatesParseTheirOwnArgv(t *testing.T) {
	root := repoRoot(t)
	checked := 0
	for _, dir := range commandDirs(t) {
		for _, path := range templatesIn(t, dir) {
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), "app.BaseCommand") {
				continue
			}
			checked++

			rel, _ := filepath.Rel(root, path)
			t.Run(rel, func(t *testing.T) {
				// Comments stripped: one explaining the rule names what it bans.
				src := codeOf(t, string(body))
				for _, banned := range []string{"getopt.Parse(", "getopt.CommandLine", "os.Args"} {
					if strings.Contains(src, banned) {
						t.Errorf("uses %s — the global flag set is shared with every other command in the binary", banned)
					}
				}
				for _, required := range []string{"getopt.New()", "app.Args(ctx)", "app.ErrUsage"} {
					if !strings.Contains(src, required) {
						t.Errorf("does not use %s, which docs/core/commands.md requires", required)
					}
				}
				if !strings.Contains(src, "Usage(ctx context.Context) error") {
					t.Error("Usage does not take a context, so it shadows app.BaseCommand.Usage instead of implementing it")
				}
			})
		}
	}
	if checked == 0 {
		t.Fatal("no command templates found — the marker moved")
	}
}

// Comments dropped, so a text rule matches what the file does rather than what
// it says about itself.
func codeOf(t *testing.T, body string) string {
	t.Helper()
	tpl, err := template.New("cmd").Funcs(funcs()).Parse(body)
	if err != nil {
		t.Fatalf("template does not parse: %v", err)
	}
	var rendered bytes.Buffer
	if err := tpl.Execute(&rendered, values(referencedFields(body), body, true)); err != nil {
		t.Fatalf("render: %v", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "rendered.go", rendered.String(), 0)
	if err != nil {
		t.Fatalf("rendered output is not Go: %v\n%s", err, rendered.String())
	}
	var out bytes.Buffer
	if err := printer.Fprint(&out, fset, file); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
