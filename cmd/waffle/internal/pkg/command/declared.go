package command

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

// Declares reports the file in dir already declaring a type of this name.
//
// Skipping by file path answers a narrower question than the one that matters:
// model/facility.go declaring Coordinates leaves model/coordinates.go absent
// and model.Coordinates taken, so writing the second one gives the package two
// declarations and no build.
func (p *Project) Declares(dir, name string) (where string, found bool) {
	entries, err := os.ReadDir(p.Path(dir))
	if err != nil {
		return "", false
	}
	set := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		// A file that does not parse is the compiler's to report. Claiming the
		// name is taken here would skip a file the caller needs.
		file, err := parser.ParseFile(set, p.Path(path), nil, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		if declaresType(file, name) {
			return path, true
		}
	}
	return "", false
}

func declaresType(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		generic, ok := decl.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, spec := range generic.Specs {
			if typed, ok := spec.(*ast.TypeSpec); ok && typed.Name.Name == name {
				return true
			}
		}
	}
	return false
}

// KeptAs reports a declaration left alone because it is already there, naming
// where it was found — the file it is in is rarely the one that was asked for.
func KeptAs(name, where string) {
	_, _ = fmt.Fprintf(output, "  %s %s — declared in %s, kept\n", keepMark, name, filepath.ToSlash(where))
}
