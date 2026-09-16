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

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

// Nothing compiles or parses the scaffolder's templates, so a broken one is
// found by whoever runs the command next. Parameter names come from the
// command's source, not from the template: read off the template, a typo would
// define the very field it misspells.

var (
	// {{ .X }}, {{ title .X }}, {{- if .X }}
	tplField = regexp.MustCompile(`\{\{-?\s*(?:\w+\s+)?\.(\w+)`)
	// {{ .X.Y }} — a param that is a record rather than a scalar.
	tplSubField = regexp.MustCompile(`\{\{-?\s*(?:\w+\s+)?\.(\w+)\.(\w+)`)
	tplToggle   = regexp.MustCompile(`\{\{-?\s*if\s+\.(\w+)`)
	// The block, because what a list holds is decided by how the body reads it:
	// a bare {{.}} wants strings, {{.Field}} wants records.
	tplRange = regexp.MustCompile(`(?s)\{\{-?\s*range\s+\.(\w+)\s*-?\}\}(.*?)\{\{-?\s*end\s*-?\}\}`)
	tplDot   = regexp.MustCompile(`\{\{-?\s*\.\s*-?\}\}`)
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
	return matching(t, dir, ".go.tpl", ".gofrag.tpl")
}

func matching(t *testing.T, dir string, suffixes ...string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		for _, suffix := range suffixes {
			if strings.HasSuffix(path, suffix) {
				out = append(out, path)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// A .gofrag.tpl is appended to a file somebody else owns, so it has no package
// clause. The suffix is what says so — a .go.tpl must parse on its own.
func asFile(path, rendered string) string {
	if strings.HasSuffix(path, ".gofrag.tpl") {
		return "package fragment\n\n" + rendered
	}
	return rendered
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
	// A record, because the scalar above fails on the first field access.
	for _, m := range tplSubField.FindAllStringSubmatch(body, -1) {
		record, ok := out[m[1]].(map[string]any)
		if !ok {
			record = map[string]any{}
			out[m[1]] = record
		}
		record[m[2]] = "thing"
	}
	for _, m := range tplToggle.FindAllStringSubmatch(body, -1) {
		out[m[1]] = toggles
	}
	// After the toggles, because a list is both: an empty one renders the off
	// path of an `if` the way false does, and a full one the on path.
	for _, m := range tplRange.FindAllStringSubmatch(body, -1) {
		out[m[1]] = list(fields, m[2], toggles)
	}
	return out
}

func list(fields []string, block string, filled bool) any {
	if !filled {
		return []any{}
	}
	if tplDot.MatchString(block) {
		return []string{"thing"}
	}
	// One record carrying every field the template names, so whichever of them
	// the body reads resolves.
	item := map[string]any{}
	for _, f := range fields {
		item[f] = "thing"
	}
	return []map[string]any{item}
}

func funcs() template.FuncMap {
	return command.Funcs()
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
					src := asFile(path, out.String())
					if _, err := parser.ParseFile(token.NewFileSet(), "rendered.go", src, 0); err != nil {
						t.Errorf("toggles=%v: rendered output is not Go: %v\n%s", toggles, err, src)
					}
				}
			})
		}
	}
}

// ValidateName accepts order_item, and an unfolded template spells it Order_item.
func TestTemplatesFoldMultiWordNames(t *testing.T) {
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
				var out bytes.Buffer
				if err := tpl.Execute(&out, multiWord(referencedFields(string(body)), string(body))); err != nil {
					t.Fatalf("render: %v", err)
				}
				for _, name := range identifiersIn(t, asFile(path, out.String())) {
					// The sentinel, not any underscore: `package {{.Package}}_test`
					// writes its own.
					if strings.Contains(name, multiWordOperand) {
						t.Errorf("renders the identifier %s — the operand is not folded", name)
					}
				}
			})
		}
	}
}

const multiWordOperand = "two_words"

// Only the operands ValidateName accepts: a .Package is a directory that exists.
func multiWord(fields []string, body string) map[string]any {
	out := values(fields, body, true)
	for _, operand := range []string{"Domain", "Entity", "Event"} {
		if _, ok := out[operand]; ok {
			out[operand] = multiWordOperand
		}
	}
	return out
}

func identifiersIn(t *testing.T, src string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "rendered.go", src, 0)
	if err != nil {
		t.Fatalf("rendered output is not Go: %v\n%s", err, src)
	}
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Name != "_" {
			out = append(out, ident.Name)
		}
		return true
	})
	return out
}

// Not Go, and the reason ValidateName refuses a hyphen: order-item is not a
// proto identifier.
var (
	protoPackage = regexp.MustCompile(`(?m)^package\s+(\S*);`)
	protoService = regexp.MustCompile(`(?m)^service\s+(\S*)\s*\{`)
	protoIdent   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func TestProtoTemplatesTakeEveryNameValidateNameAccepts(t *testing.T) {
	root := repoRoot(t)
	checked := 0
	for _, dir := range commandDirs(t) {
		for _, path := range matching(t, dir, ".proto.tpl") {
			rel, _ := filepath.Rel(root, path)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			tpl, err := template.New(filepath.Base(path)).Funcs(funcs()).Parse(string(body))
			if err != nil {
				t.Fatalf("%s does not parse: %v", rel, err)
			}

			for _, domain := range []string{"user", "order_item", "u2"} {
				if err := command.ValidateName("domain", domain); err != nil {
					t.Fatalf("the corpus is wrong: %v", err)
				}
				var out bytes.Buffer
				if err := tpl.Execute(&out, map[string]any{"Domain": domain, "Project": "example.com/svc"}); err != nil {
					t.Fatalf("%s: render: %v", rel, err)
				}
				for what, found := range map[string]*regexp.Regexp{"package": protoPackage, "service": protoService} {
					m := found.FindStringSubmatch(out.String())
					if m == nil {
						continue
					}
					checked++
					if !protoIdent.MatchString(m[1]) {
						t.Errorf("%s renders %s %q for the domain %q, which is not a proto identifier",
							rel, what, m[1], domain)
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Error("no proto declaration was checked — the templates moved, not the rule")
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
