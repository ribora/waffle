package envread

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"gitlab.com/gorib/waffle/internal/diogenfile"
)

const envPath = "gitlab.com/gorib/env"

// Every reader takes the key first, so the list is the package's, not a selection.
var readers = map[string]bool{
	"Value": true, "Array": true, "Json": true,
	"NeedValue": true, "NeedArray": true, "NeedJson": true,
	"MustValue": true, "MustArray": true, "MustJson": true,
}

// Read is one call of a reader of gitlab.com/gorib/env whose key is a literal.
type Read struct {
	Reader  string // Value, NeedArray, MustJson…
	Key     string
	Type    ast.Expr // the call's one type argument; nil without exactly one
	Call    *ast.CallExpr
	Pos     token.Position
	Imports map[string]string // the file's, alias -> path
}

// Of reports the read that node is, if it is one.
func Of(fset *token.FileSet, node ast.Node, imports map[string]string) (Read, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return Read{}, false
	}
	fun, typ := call.Fun, ast.Expr(nil)
	switch generic := fun.(type) {
	case *ast.IndexExpr:
		fun, typ = generic.X, generic.Index
	case *ast.IndexListExpr:
		fun = generic.X
	}
	selector, ok := fun.(*ast.SelectorExpr)
	if !ok || !readers[selector.Sel.Name] {
		return Read{}, false
	}
	if x, ok := selector.X.(*ast.Ident); !ok || imports[x.Name] != envPath {
		return Read{}, false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return Read{}, false
	}
	key, err := strconv.Unquote(lit.Value)
	if err != nil || key == "" {
		return Read{}, false
	}
	return Read{Reader: selector.Sel.Name, Key: key, Type: typ, Call: call, Pos: fset.Position(call.Pos()), Imports: imports}, true
}

// Module reports every read in root's config, domain and cmd, in file order,
// leaving out generated files and tests.
func Module(root string) []Read {
	fset := token.NewFileSet()
	var out []Read
	for _, dir := range []string{"config", "domain", "cmd"} {
		_ = filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			switch {
			case err != nil, entry.IsDir(), !strings.HasSuffix(path, ".go"):
				return nil
			case strings.HasSuffix(path, "_gen.go"), strings.HasSuffix(path, "_test.go"):
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return nil
			}
			imports := diogenfile.ImportAliases(file)
			ast.Inspect(file, func(n ast.Node) bool {
				if read, ok := Of(fset, n, imports); ok {
					out = append(out, read)
				}
				return true
			})
			return nil
		})
	}
	return out
}
