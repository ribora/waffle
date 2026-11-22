package boundaries

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ribora/waffle/internal/gomod"
)

func Check(opts ...Option) error {
	root, err := gomod.Root()
	if err != nil {
		return err
	}
	found, err := Inspect(root, opts...)
	if err != nil {
		return err
	}
	return Verify(found)
}

type Option func(*settings)

type settings struct {
	roots []string
}

// Roots names composition roots that carry no //go:generate diogen line for the
// check to find them by — a service that assembles its container by hand.
func Roots(dirs ...string) Option {
	return func(s *settings) { s.roots = append(s.roots, dirs...) }
}

type Crossing struct {
	From  string
	To    string
	Pos   string
	Rule  Rule
	Legal bool
}

func (c Crossing) String() string { return c.From + " -> " + c.To }

type Rule int

const (
	Domains Rule = iota
	Outside
	Wiring
	Layers
)

const (
	domainDir = "domain"
	adapter   = "adapter"
	model     = "model"
	service   = "service"
	cmd       = "cmd"
	generator = "diogen"
)

// diogen's own default, resolved from the module root like any relative -output.
const defaultOutput = "di_gen.go"

// The harness reaches domain/*/cmd/migrate for the schema it boots against, and
// nothing in the code says so.
var exempt = []string{"tests"}

func Inspect(root string, opts ...Option) ([]Crossing, error) {
	module, err := gomod.Path(root)
	if err != nil {
		return nil, err
	}
	var declared settings
	for _, opt := range opts {
		opt(&declared)
	}
	fset := token.NewFileSet()
	out, err := walk(fset, root, module, declared.roots)
	if err != nil {
		return nil, err
	}
	found, err := wires(fset, root, module)
	if err != nil {
		return nil, err
	}
	return append(out, found...), nil
}

func Verify(crossings []Crossing) error {
	refused := map[Rule][]Crossing{}
	for _, c := range crossings {
		if c.Legal {
			continue
		}
		refused[c.Rule] = append(refused[c.Rule], c)
	}
	if len(refused) == 0 {
		return nil
	}
	return errors.New(refusal(refused))
}

// Direct imports: under the transitive closure everything that reaches an
// adapter would inherit its crossing.
func walk(fset *token.FileSet, root, module string, declared []string) ([]Crossing, error) {
	var out []Crossing
	roots := map[string]bool{}
	for _, dir := range declared {
		roots[path.Clean(filepath.ToSlash(dir))] = true
	}
	err := filepath.WalkDir(root, func(file string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			if file != root && (skipDir(d.Name()) || exempted(root, file)) {
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
		parsed, err := parser.ParseFile(fset, file, nil,
			parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", rel, err)
		}

		from := path.Dir(rel)
		if output, generated := container(parsed); generated {
			roots[from] = true
			roots[path.Dir(output)] = true
		}
		for _, spec := range parsed.Imports {
			to, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("%s: import path %s does not parse", rel, spec.Path.Value)
			}
			inside, ok := strings.CutPrefix(to, module+"/")
			if !ok {
				continue
			}
			crossing, ok := classify(from, inside)
			if !ok {
				continue
			}
			crossing.Pos = fmt.Sprintf("%s:%d", rel, fset.Position(spec.Pos()).Line)
			out = append(out, crossing)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return exceptComposition(out, roots), nil
}

func container(file *ast.File) (output string, found bool) {
	for _, group := range file.Comments {
		for _, line := range group.List {
			rest, directive := strings.CutPrefix(line.Text, "//go:generate")
			if !directive {
				continue
			}
			fields := strings.Fields(rest)
			if !slices.ContainsFunc(fields, func(f string) bool {
				return f == generator || strings.HasSuffix(f, "/"+generator)
			}) {
				continue
			}
			output, named := flagValue(fields, "-output")
			if !named {
				output = defaultOutput
			}
			return output, true
		}
	}
	return "", false
}

func flagValue(fields []string, name string) (string, bool) {
	for i, f := range fields {
		for _, flag := range []string{name, "-" + name} {
			switch {
			case f == flag && i+1 < len(fields):
				return fields[i+1], true
			case strings.HasPrefix(f, flag+"="):
				return strings.TrimPrefix(f, flag+"="), true
			}
		}
	}
	return "", false
}

func exceptComposition(crossings []Crossing, roots map[string]bool) []Crossing {
	kept := make([]Crossing, 0, len(crossings))
	for _, c := range crossings {
		if roots[c.From] {
			continue
		}
		kept = append(kept, c)
	}
	return kept
}

func skipDir(name string) bool {
	return name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".")
}

func classify(from, to string) (Crossing, bool) {
	toDomain, toLayer, toInDomain := split(to)
	if !toInDomain {
		return Crossing{}, false
	}
	fromDomain, layer, fromInDomain := split(from)
	if !fromInDomain {
		return Crossing{From: from, To: to, Rule: Outside}, true
	}
	if fromDomain == toDomain {
		return withinDomain(from, to, layer, toLayer)
	}
	legal := layer == adapter && to == path.Join(domainDir, toDomain, model)
	return Crossing{From: from, To: to, Rule: Domains, Legal: legal}, true
}

func withinDomain(from, to, fromLayer, toLayer string) (Crossing, bool) {
	var legal bool
	switch toLayer {
	case cmd:
		legal = fromLayer == cmd
	case adapter:
		legal = fromLayer == adapter
	default:
		return Crossing{}, false
	}
	return Crossing{From: from, To: to, Rule: Layers, Legal: legal}, true
}

func exempted(root, dir string) bool {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	return slices.Contains(exempt, filepath.ToSlash(rel))
}

func split(pkg string) (domain, layer string, ok bool) {
	rest, ok := strings.CutPrefix(pkg, domainDir+"/")
	if !ok || rest == "" {
		return "", "", false
	}
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) == 1 {
		return parts[0], "", true
	}
	return parts[0], parts[1], true
}

var refusals = []struct {
	rule      Rule
	one, many string
	advice    string
}{
	{
		rule: Domains,
		one:  "import crosses a domain boundary outside an adapter",
		many: "imports cross a domain boundary outside an adapter",
		advice: "Only domain/<x>/adapter/** may reach a neighbour, and only its model package.\n" +
			"Declare the interface you need in your own adapter, import the neighbour's model\n" +
			"for the types in it, and translate them into your own.\n",
	},
	{
		rule: Outside,
		one:  "import reaches into a domain from outside every domain",
		many: "imports reach into a domain from outside every domain",
		advice: "A domain is reached from another domain's adapter/, from a composition root\n" +
			"and from tests/ — nowhere else. pkg/ is the usual offender: it is plumbing every\n" +
			"domain may import, so domain logic parked there is a boundary nobody can see.\n" +
			"Move it into the domain it belongs to.\n",
	},
	{
		rule: Layers,
		one:  "import reaches a layer of its own domain that is closed to it",
		many: "imports reach a layer of their own domain that is closed to them",
		advice: "Naming an adapter is how a seam is lost: adapter/ is reached only from\n" +
			"another adapter, and cmd/ only from inside cmd/. Everything else takes what\n" +
			"it needs from model/, or declares the interface it needs and lets the\n" +
			"composition root answer it.\n",
	},
	{
		rule: Wiring,
		one:  "registration answers one domain with another outside an adapter",
		many: "registrations answer one domain with another outside an adapter",
		advice: "A neighbour's behaviour arrives through its service, wired to an interface your\n" +
			"own adapter declares. Anything else couples the two domains with nowhere left to\n" +
			"translate — and leaves no import to say so.\n",
	},
}

func refusal(refused map[Rule][]Crossing) string {
	var b strings.Builder
	for _, r := range refusals {
		found := refused[r.rule]
		if len(found) == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		_, _ = fmt.Fprintf(&b, "%d %s:\n", len(found), plural(len(found), r.one, r.many))
		for _, c := range found {
			_, _ = fmt.Fprintf(&b, "\n    %s\n      %s\n", c, c.Pos)
		}
		b.WriteString("\n" + r.advice)
		if r.rule == Outside && assembles(found) {
			b.WriteString("\nIf one of these is a composition root, there is no //go:generate diogen line\n" +
				"to find it by — name it in boundaries.Roots(...) where the check is called.\n")
		}
	}
	return b.String()
}

func assembles(crossings []Crossing) bool {
	for _, c := range crossings {
		if _, layer, ok := split(c.To); ok && layer != model {
			return true
		}
	}
	return false
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
