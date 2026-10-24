package container

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
)

const envPath = "gitlab.com/gorib/env"

// Every reader takes the key first, so the list is the package's, not a selection.
var readers = map[string]bool{
	"Value": true, "Array": true, "Json": true,
	"NeedValue": true, "NeedArray": true, "NeedJson": true,
	"MustValue": true, "MustArray": true, "MustJson": true,
}

// EnvRead is one call of a reader of gitlab.com/gorib/env whose key is a literal.
type EnvRead struct {
	Reader  string // Value, NeedArray, MustJson…
	Key     string
	Type    ast.Expr // the call's one type argument; nil without exactly one
	Call    *ast.CallExpr
	Pos     token.Position
	Imports map[string]string // the file's, alias -> path
}

// EnvReadOf reports the read that node is, if it is one.
func EnvReadOf(fset *token.FileSet, node ast.Node, imports map[string]string) (EnvRead, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return EnvRead{}, false
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
		return EnvRead{}, false
	}
	if x, ok := selector.X.(*ast.Ident); !ok || imports[x.Name] != envPath {
		return EnvRead{}, false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return EnvRead{}, false
	}
	key, err := strconv.Unquote(lit.Value)
	if err != nil || key == "" {
		return EnvRead{}, false
	}
	return EnvRead{Reader: selector.Sel.Name, Key: key, Type: typ, Call: call, Pos: fset.Position(call.Pos()), Imports: imports}, true
}

// EnvReads reports every read in root's config, domain and cmd, in file order,
// leaving out generated files and tests.
func EnvReads(root string) []EnvRead {
	fset := token.NewFileSet()
	var out []EnvRead
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
			imports := ImportAliases(file)
			ast.Inspect(file, func(n ast.Node) bool {
				if read, ok := EnvReadOf(fset, n, imports); ok {
					out = append(out, read)
				}
				return true
			})
			return nil
		})
	}
	return out
}
