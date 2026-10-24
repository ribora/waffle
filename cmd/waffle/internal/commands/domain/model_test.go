package domain

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/repository"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command/commandtest"
)

// Parsed rather than built: a build installs store and diogen, which the short suite cannot.
func TestEveryModelNameTheDomainUsesIsDeclaredOnce(t *testing.T) {
	p := commandtest.Project(t, project)
	if err := Generate(p, subject, true); err != nil {
		t.Fatal(err)
	}
	if err := repository.GenerateEntity(p, subject, "address"); err != nil {
		t.Fatal(err)
	}

	modelDir := filepath.Join("domain", subject, "model")
	declared := map[string][]string{}
	for path, file := range goFiles(t, modelDir) {
		for _, name := range topLevelNames(file) {
			declared[name] = append(declared[name], path)
		}
	}
	for name, paths := range declared {
		if len(paths) > 1 {
			t.Errorf("model.%s is declared %d times: %v", name, len(paths), paths)
		}
	}

	modelImport := strconv.Quote(project + "/domain/" + subject + "/model")
	for path, file := range goFiles(t, ".") {
		for _, imp := range file.Imports {
			if imp.Path.Value != modelImport {
				continue
			}
			local := "model"
			if imp.Name != nil {
				local = imp.Name.Name
			}
			ast.Inspect(file, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == local && declared[sel.Sel.Name] == nil {
					t.Errorf("%s uses model.%s, which the model does not declare", filepath.ToSlash(path), sel.Sel.Name)
				}
				return true
			})
		}
	}
}

func goFiles(t *testing.T, root string) map[string]*ast.File {
	t.Helper()
	set := token.NewFileSet()
	out := map[string]*ast.File{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		file, err := parser.ParseFile(set, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		out[path] = file
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func topLevelNames(file *ast.File) []string {
	var out []string
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Recv == nil {
				out = append(out, decl.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					out = append(out, spec.Name.Name)
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						out = append(out, name.Name)
					}
				}
			}
		}
	}
	return out
}
