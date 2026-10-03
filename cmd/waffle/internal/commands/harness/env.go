package harness

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"gitlab.com/gorib/waffle/internal/diogenfile"
)

const envPath = "gitlab.com/gorib/env"

// One variable, as a field of the generated Variables. A Need* read has no
// value of its own, so the harness always sets one; a read with a fallback is
// set only when the env hook changed it, because the fallback is the container's
// own answer and writing it back does not always round-trip — an empty
// []string{} comes back from the environment as [""].
type variable struct {
	Key     string
	Type    string
	Default string // Go source; empty for the zero value
	List    bool
	Always  bool
	imports map[string]string // alias -> path the type and default need
}

type envRead struct {
	variable
	guards []string // Replacements fields whose constructor makes the read
}

var (
	needReaders     = map[string]bool{"NeedValue": true, "NeedArray": true, "NeedJson": true}
	fallbackReaders = map[string]bool{"Value": true, "Array": true, "Json": true}
	durationUnits   = map[string]bool{"Nanosecond": true, "Microsecond": true, "Millisecond": true, "Second": true, "Minute": true, "Hour": true}
)

// Walked with the guards that enclose each read, so a read under a filled seam
// is known to be skipped. owners is nil when only the keys are wanted.
func envReads(file *ast.File, root ast.Node, owners func([]string) []string) []envRead {
	aliases := diogenfile.ImportAliases(file)
	var out []envRead
	var walk func(n ast.Node, guards []string)
	walk = func(n ast.Node, guards []string) {
		ast.Inspect(n, func(node ast.Node) bool {
			if node == nil {
				return false
			}
			if guarded, ok := node.(*ast.IfStmt); ok && node != n && owners != nil {
				if fields := diogenfile.GuardedFields(guarded.Cond); len(fields) > 0 {
					if guarded.Init != nil {
						walk(guarded.Init, guards)
					}
					walk(guarded.Body, append(slices.Clone(guards), owners(fields)...))
					if guarded.Else != nil {
						walk(guarded.Else, guards)
					}
					return false
				}
			}
			if read, ok := readEnv(node, aliases); ok {
				read.guards = guards
				out = append(out, read)
			}
			return true
		})
	}
	walk(root, nil)
	return out
}

// Every read in the module the container does not make: config, domain and cmd,
// the directories `waffle agents` reads its list from too.
func moduleReads() []envRead {
	var out []envRead
	for _, root := range []string{"config", "domain", "cmd"} {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			switch {
			case err != nil, info.IsDir(), !strings.HasSuffix(path, ".go"):
				return nil
			case strings.HasSuffix(path, "_gen.go"), strings.HasSuffix(path, "_test.go"):
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
			if err != nil {
				return nil
			}
			out = append(out, envReads(file, file, nil)...)
			return nil
		})
	}
	return out
}

func readEnv(node ast.Node, aliases map[string]string) (envRead, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return envRead{}, false
	}
	fun, typeArg := call.Fun, ast.Expr(nil)
	if index, ok := fun.(*ast.IndexExpr); ok {
		fun, typeArg = index.X, index.Index
	}
	selector, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return envRead{}, false
	}
	if x, ok := selector.X.(*ast.Ident); !ok || aliases[x.Name] != envPath {
		return envRead{}, false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return envRead{}, false
	}
	key, err := strconv.Unquote(lit.Value)
	// Custom is the field for what has no field of its own.
	if err != nil || !token.IsIdentifier(key) || key == "Custom" {
		return envRead{}, false
	}
	name := selector.Sel.Name
	v := variable{Key: key, List: strings.HasSuffix(name, "Array"), Always: needReaders[name], imports: map[string]string{}}

	switch {
	case name == "NeedJson":
		v.Type, v.Default = "string", `"null"`
		return envRead{variable: v}, true
	case name == "Json":
		// The raw JSON; empty is not JSON, so the container keeps its fallback.
		v.Type = "string"
		return envRead{variable: v}, true
	case needReaders[name]:
		elem, ok := v.source(typeArg, aliases)
		if !ok {
			return envRead{}, false
		}
		v.Type, v.Default = elem, placeholder(key, elem)
		if v.List {
			v.Type = "[]" + elem
			if v.Default != "" {
				v.Default = v.Type + "{" + v.Default + "}"
			}
		}
		return envRead{variable: v}, true
	case fallbackReaders[name] && len(call.Args) == 2:
		fallback := call.Args[1]
		if !v.contained(fallback, aliases) {
			return envRead{}, false
		}
		typ, ok := v.source(typeArg, aliases)
		if !ok {
			typ, ok = v.infer(fallback, aliases)
		}
		if !ok {
			return envRead{}, false
		}
		v.Type, v.Default = typ, render(fallback)
		if v.List && !strings.HasPrefix(typ, "[]") {
			v.Type = "[]" + typ
		}
		return envRead{variable: v}, true
	}
	return envRead{}, false
}

// The type as source, with whatever package it names imported.
func (v *variable) source(expr ast.Expr, aliases map[string]string) (string, bool) {
	if expr == nil || !v.contained(expr, aliases) {
		return "", false
	}
	return render(expr), true
}

// What a fallback's type is, when the call leaves it to inference.
func (v *variable) infer(expr ast.Expr, aliases map[string]string) (string, bool) {
	if composite, ok := expr.(*ast.CompositeLit); ok && composite.Type != nil {
		return v.source(composite.Type, aliases)
	}
	duration := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if selector, ok := n.(*ast.SelectorExpr); ok {
			if x, ok := selector.X.(*ast.Ident); ok && aliases[x.Name] == "time" && durationUnits[selector.Sel.Name] {
				duration = true
			}
		}
		return !duration
	})
	if duration {
		for alias, p := range aliases {
			if p == "time" {
				return alias + ".Duration", true
			}
		}
	}
	var out string
	ast.Inspect(expr, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.BasicLit:
			out = map[token.Token]string{token.STRING: "string", token.INT: "int", token.FLOAT: "float64", token.CHAR: "rune"}[t.Kind]
		case *ast.Ident:
			if t.Name == "true" || t.Name == "false" {
				out = "bool"
			}
		}
		return out == ""
	})
	return out, out != ""
}

// Only literals, operators and other packages' exported names: anything that
// names a declaration of its own file cannot be copied out of it.
func (v *variable) contained(expr ast.Expr, aliases map[string]string) bool {
	ok := true
	ast.Inspect(expr, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.SelectorExpr:
			x, isIdent := t.X.(*ast.Ident)
			if !isIdent || aliases[x.Name] == "" {
				ok = false
				return false
			}
			v.imports[x.Name] = aliases[x.Name]
			return false
		case *ast.Ident:
			if !predeclared[t.Name] {
				ok = false
			}
		case *ast.FuncLit, *ast.CallExpr:
			if call, isCall := t.(*ast.CallExpr); isCall && len(call.Args) == 1 {
				return true // a conversion
			}
			ok = false
		}
		return ok
	})
	return ok
}

var predeclared = map[string]bool{
	"true": true, "false": true, "nil": true,
	"string": true, "bool": true, "rune": true, "byte": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true,
}

// A string names its own key, so a call that reaches it says which variable it
// came from. Everything else is its zero value, which parses as its type.
func placeholder(key, typ string) string {
	if typ != "string" {
		return ""
	}
	return strconv.Quote(strings.ToLower(strings.ReplaceAll(key, "_", "-")) + ".harness.invalid")
}

func render(expr ast.Expr) string {
	var out bytes.Buffer
	if err := format.Node(&out, token.NewFileSet(), expr); err != nil {
		return ""
	}
	return out.String()
}

// The container's reads decide a key they make, skipped or not; a key only the
// rest of the module reads is always read.
func (g graph) variables(filled map[string]bool, module []envRead) []variable {
	var out []variable
	seen := map[string]bool{}
	for _, read := range g.reads {
		seen[read.Key] = true
	}
	taken := map[string]bool{}
	for _, read := range g.reads {
		if taken[read.Key] || slices.ContainsFunc(read.guards, func(seam string) bool { return filled[seam] }) {
			continue
		}
		taken[read.Key] = true
		out = append(out, read.variable)
	}
	for _, read := range module {
		if !seen[read.Key] {
			seen[read.Key] = true
			out = append(out, read.variable)
		}
	}
	slices.SortFunc(out, func(a, b variable) int { return strings.Compare(a.Key, b.Key) })
	return out
}
