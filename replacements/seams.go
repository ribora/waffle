package replacements

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"slices"
	"strconv"
)

type TypeRef struct {
	Path string
	Name string
}

func (t TypeRef) String() string { return t.Path + "." + t.Name }

// More than one type means diogen merged registrations sharing the field, and
// the entry is a composite interface that no single mock satisfies.
type Seam struct {
	Field string
	Types []TypeRef
	// Replacing a leaf leaves the whole service under test; replacing an interior
	// seam cuts out everything below it.
	Leaf bool
}

func (s Seam) Composite() bool { return len(s.Types) > 1 }

// Parses instead of type-checking, so it works on a service whose dependencies
// do not currently compile.
func ReadSeams(c Container, importPath string) ([]Seam, error) {
	file, err := parser.ParseFile(token.NewFileSet(), c.File, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", c.File, err)
	}
	fields, err := replacementFields(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", c.File, err)
	}

	imports := importAliases(file)
	leaves := leafSeams(file)
	seams := make([]Seam, 0, len(fields))
	for _, field := range fields {
		if len(field.Names) != 1 {
			continue
		}
		name := field.Names[0].Name
		types, err := typeRefs(field.Type, imports, importPath)
		if err != nil {
			return nil, fmt.Errorf("%s: field %s: %w", c.File, name, err)
		}
		seams = append(seams, Seam{Field: name, Types: types, Leaf: leaves[name]})
	}
	return seams, nil
}

// A guarded block that reads no other container field is building from the
// environment alone — a leaf. Shape of the generated body: di/cmd/diogen/ARCHITECTURE.md.
func leafSeams(file *ast.File) map[string]bool {
	body := initAppWithBody(file)
	if body == nil {
		return nil
	}

	fields := map[string][]string{} // replacements field -> container fields
	owner := map[string]string{}    // container field -> replacements field
	for _, stmt := range body.List {
		field, seam, ok := prologueAssignment(stmt)
		if !ok {
			continue
		}
		fields[seam] = append(fields[seam], field)
		owner[field] = seam
	}

	reads := map[string]map[string]bool{}
	for _, stmt := range body.List {
		block, ok := stmt.(*ast.IfStmt)
		if !ok {
			continue
		}
		for _, guarded := range guardedFields(block.Cond) {
			seam, known := owner[guarded]
			if !known {
				continue
			}
			if reads[seam] == nil {
				reads[seam] = map[string]bool{}
			}
			for read := range containerReads(block.Body) {
				reads[seam][read] = true
			}
		}
	}

	out := make(map[string]bool, len(fields))
	for seam, own := range fields {
		leaf := true
		for read := range reads[seam] {
			if !slices.Contains(own, read) {
				leaf = false
				break
			}
		}
		out[seam] = leaf
	}
	return out
}

func initAppWithBody(file *ast.File) *ast.BlockStmt {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == "InitAppWith" {
			return fn.Body
		}
	}
	return nil
}

// container.x = replacements.X
func prologueAssignment(stmt ast.Stmt) (field, seam string, ok bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return "", "", false
	}
	field, ok = qualifiedBy(assign.Lhs[0], "container")
	if !ok {
		return "", "", false
	}
	seam, ok = qualifiedBy(assign.Rhs[0], "replacements")
	if !ok {
		return "", "", false
	}
	return field, seam, true
}

// Recursive, so a guard carrying a branch condition alongside still resolves.
func guardedFields(cond ast.Expr) []string {
	binary, ok := cond.(*ast.BinaryExpr)
	if !ok {
		return nil
	}
	switch binary.Op {
	case token.LAND, token.LOR:
		return append(guardedFields(binary.X), guardedFields(binary.Y)...)
	case token.EQL:
		if ident, ok := binary.Y.(*ast.Ident); !ok || ident.Name != "nil" {
			return nil
		}
		if field, ok := qualifiedBy(binary.X, "container"); ok {
			return []string{field}
		}
	}
	return nil
}

// An assignment target is a write, and the closers slice is bookkeeping the
// generator adds to every seam alike.
func containerReads(block *ast.BlockStmt) map[string]bool {
	written := map[ast.Expr]bool{}
	ast.Inspect(block, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			for _, lhs := range assign.Lhs {
				written[lhs] = true
			}
		}
		return true
	})

	out := map[string]bool{}
	ast.Inspect(block, func(n ast.Node) bool {
		expr, ok := n.(ast.Expr)
		if !ok || written[expr] {
			return true
		}
		if field, ok := qualifiedBy(expr, "container"); ok && field != "closers" {
			out[field] = true
		}
		return true
	})
	return out
}

func qualifiedBy(expr ast.Expr, ident string) (string, bool) {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	x, ok := selector.X.(*ast.Ident)
	if !ok || x.Name != ident {
		return "", false
	}
	return selector.Sel.Name, true
}

func replacementFields(file *ast.File) ([]*ast.Field, error) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "Replacements" {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return nil, errors.New("the Replacements type is not a struct")
			}
			return st.Fields.List, nil
		}
	}
	return nil, errors.New("no Replacements type — regenerate with a diogen that emits replacement seams")
}

// diogen writes an explicit alias on every non-stdlib import, so the fallback to
// the last path segment is for a file that was not written by it.
func importAliases(file *ast.File) map[string]string {
	out := make(map[string]string, len(file.Imports))
	for _, imp := range file.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(p)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		out[name] = p
	}
	return out
}

func typeRefs(expr ast.Expr, aliases map[string]string, self string) ([]TypeRef, error) {
	switch t := expr.(type) {
	case *ast.InterfaceType:
		var out []TypeRef
		for _, embedded := range t.Methods.List {
			if len(embedded.Names) > 0 {
				return nil, errors.New("a composite seam holding a method rather than an embedded interface")
			}
			refs, err := typeRefs(embedded.Type, aliases, self)
			if err != nil {
				return nil, err
			}
			out = append(out, refs...)
		}
		return out, nil
	// An instantiated generic: the mock is of the generic interface, which is why
	// several instantiations of one type collapse to a single entry.
	case *ast.IndexExpr:
		return typeRefs(t.X, aliases, self)
	case *ast.IndexListExpr:
		return typeRefs(t.X, aliases, self)
	case *ast.SelectorExpr:
		alias, ok := t.X.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("unexpected qualifier %T", t.X)
		}
		p, known := aliases[alias.Name]
		if !known {
			return nil, fmt.Errorf("%q is not an import of the generated file", alias.Name)
		}
		return []TypeRef{{Path: p, Name: t.Sel.Name}}, nil
	case *ast.Ident:
		return []TypeRef{{Path: self, Name: t.Name}}, nil
	default:
		return nil, fmt.Errorf("unexpected type %T — every seam is an interface", expr)
	}
}
