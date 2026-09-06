package replacements

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

func Check() error {
	root, err := ModuleRoot()
	if err != nil {
		return err
	}
	containers, err := DiscoverContainers(root)
	if err != nil {
		return err
	}
	found, err := Inspect(root, containers)
	if err != nil {
		return err
	}
	cfg, err := LoadConfig(root)
	if err != nil {
		return err
	}
	_, err = Verify(root, found, cfg)
	return err
}

type Inspection struct {
	Seams []Seam
	Paths []string
	Files []string
}

// Takes the containers rather than finding them, because the CLI lets a person
// name one.
func Inspect(root string, containers []Container) (Inspection, error) {
	module, err := Module(root)
	if err != nil {
		return Inspection{}, err
	}

	var out Inspection
	for _, container := range containers {
		dir, err := filepath.Rel(root, container.Dir)
		if err != nil {
			return Inspection{}, err
		}
		importPath := path.Join(module, filepath.ToSlash(dir))
		seams, err := ReadSeams(container, importPath)
		if err != nil {
			return Inspection{}, err
		}
		file, err := filepath.Abs(container.File)
		if err != nil {
			return Inspection{}, err
		}
		out.Seams = append(out.Seams, seams...)
		out.Paths = append(out.Paths, importPath)
		out.Files = append(out.Files, file)
	}
	return out, nil
}

type Report struct {
	// Once per field, however many tests fill it in.
	Granted  int
	Warnings []string
}

// The report comes back with the denial too: a warning about the list is worth
// reading most of all when the list has just refused something.
func Verify(root string, found Inspection, cfg Config) (Report, error) {
	report := Report{Warnings: unknownGrants(cfg, found.Seams)}

	filledIn, warnings, err := scan(root, found)
	if err != nil {
		return Report{}, err
	}
	report.Warnings = append(report.Warnings, warnings...)

	byField := fields(found.Seams)
	var refused []violation
	groups := group(filledIn)
	for _, group := range groups {
		if group.Field == positional {
			return report, fmt.Errorf("%s: a Replacements literal without field names — name the fields, "+
				"or nothing can say which seams it replaces", group.Pos[0])
		}
		seam, known := byField[group.Field]
		if !known {
			return report, fmt.Errorf("%s: Replacements has no field %s — the container is out of date, "+
				"run go generate ./...", group.Pos[0], group.Field)
		}
		denied := deniedTypes(seam, cfg)
		if len(denied) == 0 {
			continue
		}
		refused = append(refused, violation{Field: group.Field, Types: denied, Where: group.Pos})
	}
	if len(refused) > 0 {
		return report, fmt.Errorf("%s", refusal(refused))
	}
	report.Granted = len(groups)
	return report, nil
}

type filled struct {
	Field string
	Pos   string
}

// An element of the literal with no field name: which seam it fills depends on
// the order of a generated struct, so the check refuses to read it.
const positional = ""

type violation struct {
	Field string
	Types []TypeRef
	Where []string
}

// Import paths in full: the answer to this message is a line of waffle.yaml, and
// that line is keyed by the whole path.
func refusal(refused []violation) string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "%d replaced %s not in replacements.allow:\n", len(refused),
		plural(len(refused), "field is", "fields are"))
	for _, v := range refused {
		names := make([]string, 0, len(v.Types))
		for _, ref := range v.Types {
			names = append(names, ref.String())
		}
		_, _ = fmt.Fprintf(&b, "\n    %s — %s\n", v.Field, strings.Join(names, " + "))
		for _, where := range v.Where {
			_, _ = fmt.Fprintf(&b, "      %s\n", where)
		}
	}
	b.WriteString("\nreplacements.allow in waffle.yaml is what a test may supply itself. It is the\n")
	b.WriteString("platform engineer's decision: report the seam you need instead of granting it.\n")
	return b.String()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// A composite field replaces every contract it merges, so one ungranted
// constituent refuses the field.
func deniedTypes(seam Seam, cfg Config) []TypeRef {
	var out []TypeRef
	for _, ref := range seam.Types {
		if !cfg.Allowed(ref.Path, ref.Name) {
			out = append(out, ref)
		}
	}
	return out
}

// Two containers can expose the same field name; the union is what has to be
// granted, since either one may be the one under test.
func fields(seams []Seam) map[string]Seam {
	out := map[string]Seam{}
	for _, seam := range seams {
		merged, seen := out[seam.Field]
		if !seen {
			out[seam.Field] = seam
			continue
		}
		// Cloned, because the two seams may share a backing array with the
		// slices ReadSeams still holds.
		types := slices.Clone(merged.Types)
		for _, ref := range seam.Types {
			if !slices.Contains(types, ref) {
				types = append(types, ref)
			}
		}
		merged.Types = types
		out[seam.Field] = merged
	}
	return out
}

// A grant that expired denies nothing, so it is a warning. Worded here, not by
// the caller: the service and the command have to say the same thing.
func unknownGrants(cfg Config, seams []Seam) []string {
	exposed := map[string]bool{}
	for _, seam := range seams {
		for _, ref := range seam.Types {
			exposed[ref.String()] = true
		}
	}

	var out []string
	for pkg, names := range cfg.Allow {
		for _, name := range names {
			ref := TypeRef{Path: pkg, Name: name}
			if !exposed[ref.String()] {
				out = append(out, fmt.Sprintf("replacements.allow grants %s, which no container exposes", ref))
			}
		}
	}
	sort.Strings(out)
	return out
}

type occurrences struct {
	Field string
	Pos   []string
}

func group(found []filled) []occurrences {
	byField := map[string][]string{}
	var order []string
	for _, f := range found {
		if _, seen := byField[f.Field]; !seen {
			order = append(order, f.Field)
		}
		byField[f.Field] = append(byField[f.Field], f.Pos)
	}
	sort.Strings(order)

	out := make([]occurrences, 0, len(order))
	for _, field := range order {
		out = append(out, occurrences{Field: field, Pos: byField[field]})
	}
	return out
}

// scan reads the module rather than one directory: a replacement is the same
// decision wherever it is made, and a unit test next to the code it pins is as
// good a place to make it as the feature package.
func scan(root string, containers Inspection) ([]filled, []string, error) {
	var out []filled
	var warnings []string
	err := filepath.WalkDir(root, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		if entry.IsDir() {
			if skipScan(rel) {
				return fs.SkipDir
			}
			return nil
		}
		// The generated container declares the struct and assigns its fields; a
		// test in the same package is read like any other.
		if !strings.HasSuffix(p, ".go") || slices.Contains(containers.Files, p) {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			// Not fatal here: a file in this state fails the build too, and the
			// compiler names the syntax better than this check can.
			warnings = append(warnings, fmt.Sprintf("%s does not parse and was not read", filepath.ToSlash(rel)))
			return nil
		}
		out = append(out, replaced(file, fset, root, imports(file), containers.Paths)...)
		return nil
	})
	return out, warnings, err
}

// The harness is the platform engineer's own code, and it is where the real
// database is handed in — constraining it with the engineer's own list would
// deny the very thing the list exists to allow.
var harnessDir = filepath.Join("tests", "internal")

func skipScan(rel string) bool {
	name := filepath.Base(rel)
	switch {
	case rel == ".":
		return false
	case name == "vendor" || name == "node_modules" || strings.HasPrefix(name, "."):
		return true
	default:
		return rel == harnessDir
	}
}

func replaced(file *ast.File, fset *token.FileSet, root string, aliases map[string]string, containers []string) []filled {
	var out []filled
	at := func(pos token.Pos) string {
		p := fset.Position(pos)
		rel, err := filepath.Rel(root, p.Filename)
		if err != nil {
			rel = p.Filename
		}
		return fmt.Sprintf("%s:%d", filepath.ToSlash(rel), p.Line)
	}

	held := holders(file, aliases, containers)
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CompositeLit:
			if !isReplacements(node.Type, aliases, containers) {
				return true
			}
			for _, elt := range node.Elts {
				kv, keyed := elt.(*ast.KeyValueExpr)
				if !keyed {
					out = append(out, filled{Field: positional, Pos: at(elt.Pos())})
					continue
				}
				if key, ok := kv.Key.(*ast.Ident); ok {
					out = append(out, filled{Field: key.Name, Pos: at(kv.Pos())})
				}
			}
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				if ident, ok := sel.X.(*ast.Ident); ok && held[ident.Name] {
					out = append(out, filled{Field: sel.Sel.Name, Pos: at(sel.Pos())})
				}
			}
		}
		return true
	})
	return out
}

func holders(file *ast.File, aliases map[string]string, containers []string) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ValueSpec:
			if isReplacements(node.Type, aliases, containers) {
				for _, name := range node.Names {
					out[name.Name] = true
				}
			}
		case *ast.Field:
			if isReplacements(node.Type, aliases, containers) {
				for _, name := range node.Names {
					out[name.Name] = true
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range node.Lhs {
				if i >= len(node.Rhs) {
					break
				}
				if !isReplacements(literalType(node.Rhs[i]), aliases, containers) {
					continue
				}
				if ident, ok := lhs.(*ast.Ident); ok {
					out[ident.Name] = true
				}
			}
		}
		return true
	})
	return out
}

func literalType(expr ast.Expr) ast.Expr {
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		expr = unary.X
	}
	if lit, ok := expr.(*ast.CompositeLit); ok {
		return lit.Type
	}
	return nil
}

const structName = "Replacements"

func isReplacements(expr ast.Expr, aliases map[string]string, containers []string) bool {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != structName {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return slices.Contains(containers, aliases[pkg.Name])
}

func imports(file *ast.File) map[string]string {
	out := make(map[string]string, len(file.Imports))
	for _, spec := range file.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(p)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		out[name] = p
	}
	return out
}
