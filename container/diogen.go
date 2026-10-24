package container

import (
	"go/ast"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"
)

func InitAppWith(file *ast.File) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == "InitAppWith" {
			return fn
		}
	}
	return nil
}

func InitAppWithBody(file *ast.File) *ast.BlockStmt {
	if fn := InitAppWith(file); fn != nil {
		return fn.Body
	}
	return nil
}

// The statements of InitAppWith and of every buildX := diogenBuild(name, func…) in it:
// diogen builds each field inside a closure of its own.
func BuildStatements(body *ast.BlockStmt) []ast.Stmt {
	stmts := slices.Clone(body.List)
	for _, stmt := range body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || assign.Tok != token.DEFINE || len(assign.Rhs) != 1 {
			continue
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			continue
		}
		if fun, ok := call.Fun.(*ast.Ident); !ok || fun.Name != "diogenBuild" {
			continue
		}
		if lit, ok := call.Args[1].(*ast.FuncLit); ok {
			stmts = append(stmts, lit.Body.List...)
		}
	}
	return stmts
}

// container.x = replacements.X
func PrologueAssignment(stmt ast.Stmt) (field, seam string, ok bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return "", "", false
	}
	field, ok = QualifiedBy(assign.Lhs[0], "container")
	if !ok {
		return "", "", false
	}
	seam, ok = QualifiedBy(assign.Rhs[0], "replacements")
	if !ok {
		return "", "", false
	}
	return field, seam, true
}

// Recursive, so a guard carrying a branch condition alongside still resolves.
func GuardedFields(cond ast.Expr) []string {
	binary, ok := cond.(*ast.BinaryExpr)
	if !ok {
		return nil
	}
	switch binary.Op {
	case token.LAND, token.LOR:
		return append(GuardedFields(binary.X), GuardedFields(binary.Y)...)
	case token.EQL:
		if ident, ok := binary.Y.(*ast.Ident); !ok || ident.Name != "nil" {
			return nil
		}
		if field, ok := QualifiedBy(binary.X, "container"); ok {
			return []string{field}
		}
	}
	return nil
}

// The locals that hold a value of a type the container cannot name: x := diogenZero1(lib.NewX).
func Locals(body *ast.BlockStmt) map[string]bool {
	out := map[string]bool{}
	for _, stmt := range body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || assign.Tok != token.DEFINE || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			continue
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		if fun, ok := call.Fun.(*ast.Ident); !ok || !strings.HasPrefix(fun.Name, "diogenZero") {
			continue
		}
		if local, ok := assign.Lhs[0].(*ast.Ident); ok {
			out[local.Name] = true
		}
	}
	return out
}

// An assignment target is a write, the closers slice is bookkeeping the generator
// adds to every seam alike, and a local stands for a field.
func ContainerReads(block *ast.BlockStmt, locals map[string]bool) map[string]bool {
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
		if field, ok := QualifiedBy(expr, "container"); ok && field != "closers" {
			out[field] = true
		}
		if ident, ok := expr.(*ast.Ident); ok && locals[ident.Name] {
			out[ident.Name] = true
		}
		return true
	})
	return out
}

// An empty qualifier matches any package: the container imports a tool under
// whatever alias diogen picked.
func QualifiedBy(expr ast.Expr, ident string) (string, bool) {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	x, ok := selector.X.(*ast.Ident)
	if !ok || (ident != "" && x.Name != ident) {
		return "", false
	}
	return selector.Sel.Name, true
}

// diogen writes an explicit alias on every non-stdlib import, so the fallback to
// the last path segment is for a file that was not written by it.
func ImportAliases(file *ast.File) map[string]string {
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
