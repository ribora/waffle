package boundaries

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const diPackage = "gitlab.com/gorib/di"

func wires(fset *token.FileSet, root, module string) ([]Crossing, error) {
	found, err := collect(fset, root, module)
	if err != nil {
		return nil, err
	}

	handles := resolveHandles(found)

	var out []Crossing
	for i := range found {
		registrar := &found[i]
		registrar.read.handles = handles[registrar.read.self]
		out = append(out, registrar.read.registrations(registrar.file, fset, registrar.rel)...)
	}
	return out, nil
}

type registrar struct {
	rel  string
	file *ast.File
	read reader
}

// The whole module, not config/di: a file registers if it imports di, and a
// service may keep more than one that does.
func collect(fset *token.FileSet, root, module string) ([]registrar, error) {
	names := &packages{root: root, module: module}

	var out []registrar
	err := filepath.WalkDir(root, func(file string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			if file != root && skipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		case !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go"):
			return nil
		}

		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		parsed, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", rel, err)
		}
		read := reader{module: module, self: path.Dir(rel)}
		read.resolve(parsed, names)
		if read.di == "" {
			return nil
		}
		out = append(out, registrar{rel: rel, file: parsed, read: read})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func resolveHandles(found []registrar) map[string]map[string]string {
	out := map[string]map[string]string{}
	for i := range found {
		registrar := &found[i]
		ast.Inspect(registrar.file, func(n ast.Node) bool {
			name, ctor, ok := defined(&registrar.read, n)
			if !ok {
				return true
			}
			pkg := registrar.read.pkg(ctor)
			if pkg == "" {
				return true
			}
			self := registrar.read.self
			if out[self] == nil {
				out[self] = map[string]string{}
			}
			// One name defined twice answers for neither: empty reads as unknown.
			if was, taken := out[self][name]; taken && was != pkg {
				pkg = ""
			}
			out[self][name] = pkg
			return true
		})
	}
	return out
}

func defined(r *reader, n ast.Node) (name string, ctor ast.Expr, ok bool) {
	var target, value ast.Expr
	switch node := n.(type) {
	case *ast.AssignStmt:
		if len(node.Lhs) != 1 || len(node.Rhs) != 1 {
			return "", nil, false
		}
		target, value = node.Lhs[0], node.Rhs[0]
	case *ast.ValueSpec:
		if len(node.Names) != 1 || len(node.Values) != 1 {
			return "", nil, false
		}
		target, value = node.Names[0], node.Values[0]
	default:
		return "", nil, false
	}

	held, ok := target.(*ast.Ident)
	if !ok {
		return "", nil, false
	}
	call, ok := value.(*ast.CallExpr)
	if !ok {
		return "", nil, false
	}
	if fn, _, ok := r.diCall(call); !ok || fn != "Define" || len(call.Args) == 0 {
		return "", nil, false
	}
	return held.Name, call.Args[0], true
}

type reader struct {
	module  string
	self    string
	imports map[string]string
	handles map[string]string
	di      string
}

func (r *reader) resolve(file *ast.File, names *packages) {
	r.imports = map[string]string{}
	for _, spec := range file.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		local := ""
		switch {
		case spec.Name != nil:
			local = spec.Name.Name
		default:
			local = names.name(p)
		}
		if local == "_" || local == "." {
			continue
		}
		if p == diPackage {
			r.di = local
			continue
		}
		inside, ok := strings.CutPrefix(p, r.module+"/")
		if !ok {
			continue
		}
		r.imports[local] = inside
	}
}

func (r *reader) registrations(file *ast.File, fset *token.FileSet, rel string) []Crossing {
	var out []Crossing
	add := func(iface, ctor ast.Expr, pos token.Pos) {
		crossing, ok := classifyWire(r.pkg(iface), r.pkg(ctor))
		if !ok {
			return
		}
		crossing.Pos = fmt.Sprintf("%s:%d", rel, fset.Position(pos).Line)
		out = append(out, crossing)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name, arg, ok := r.diCall(call)
		if !ok || len(call.Args) == 0 {
			return true
		}
		var ctor ast.Expr
		switch name {
		case "Wire":
			ctor = call.Args[0]
			add(arg, ctor, call.Pos())
		case "Define":
			ctor = call.Args[0]
		default:
			return true
		}
		for _, opt := range call.Args[1:] {
			option, ok := opt.(*ast.CallExpr)
			if !ok {
				continue
			}
			if name, alias, ok := r.diCall(option); ok && name == "Alias" {
				add(alias, ctor, option.Pos())
			}
		}
		return true
	})
	return out
}

func (r *reader) diCall(call *ast.CallExpr) (name string, arg ast.Expr, ok bool) {
	fun := call.Fun
	switch indexed := fun.(type) {
	case *ast.IndexExpr:
		arg, fun = indexed.Index, indexed.X
	case *ast.IndexListExpr:
		if len(indexed.Indices) == 0 {
			return "", nil, false
		}
		arg, fun = indexed.Indices[0], indexed.X
	}
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return "", nil, false
	}
	qualifier, ok := sel.X.(*ast.Ident)
	if !ok || qualifier.Name != r.di {
		return "", nil, false
	}
	return sel.Sel.Name, arg, true
}

// Empty when the expression names no package at all — a closure or a literal,
// neither of which carries a domain.
func (r *reader) pkg(expr ast.Expr) string {
	for {
		star, ok := expr.(*ast.StarExpr)
		if !ok {
			break
		}
		expr = star.X
	}
	switch e := expr.(type) {
	case *ast.Ident:
		if held, ok := r.handles[e.Name]; ok {
			return held
		}
		return r.self
	case *ast.SelectorExpr:
		qualifier, ok := e.X.(*ast.Ident)
		if !ok {
			return ""
		}
		return r.imports[qualifier.Name]
	}
	return ""
}

func classifyWire(consumer, provider string) (Crossing, bool) {
	consumerDomain, layer, consumerInDomain := split(consumer)
	providerDomain, _, providerInDomain := split(provider)
	if !consumerInDomain || !providerInDomain || consumerDomain == providerDomain {
		return Crossing{}, false
	}
	own := path.Join(domainDir, providerDomain, service)
	legal := layer == adapter && (provider == own || strings.HasPrefix(provider, own+"/"))
	return Crossing{From: consumer, To: provider, Rule: Wiring, Legal: legal}, true
}

type packages struct {
	root   string
	module string
	known  map[string]string
}

func (p *packages) name(importPath string) string {
	if found, ok := p.known[importPath]; ok {
		return found
	}
	name := path.Base(importPath)
	if rel, inside := strings.CutPrefix(importPath, p.module+"/"); inside {
		if declared, ok := declaredName(filepath.Join(p.root, filepath.FromSlash(rel))); ok {
			name = declared
		}
	}
	if p.known == nil {
		p.known = map[string]string{}
	}
	p.known[importPath] = name
	return name
}

func declaredName(dir string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.PackageClauseOnly)
		if err != nil {
			continue
		}
		return file.Name.Name, true
	}
	return "", false
}
