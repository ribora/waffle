package harness

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"unicode"

	"gitlab.com/gorib/waffle/replacements"
)

const (
	sqlDbPath    = "gitlab.com/gorib/sql"
	sqlDbName    = "Db"
	redisDbPath  = "gitlab.com/gorib/redis"
	redisDbName  = "Db"
	connPath     = "google.golang.org/grpc"
	connName     = "ClientConnInterface"
	loggerPath   = "gitlab.com/gorib/pry"
	loggerName   = "Logger"
	hiltonPath   = "gitlab.com/gorib/hilton"
	hiltonName   = "Provider"
	consumerPath = "gitlab.com/gorib/waffle/tools/scribus"
	consumerName = "Consumer"
)

type shape struct {
	Postgres bool
	Redis    bool
	Logger   bool
	Hilton   bool
	Consumer bool
	Dead     []conn
	Grpc     wiring
	Http     wiring
}

type wiring struct {
	Handlers     []string
	Interceptors []string
	Stream       []string
	Mux          string
	Router       string
}

func (w wiring) Present() bool { return len(w.Handlers) > 0 || w.Mux != "" }

type conn struct {
	Field string
	Name  string
}

func read(container replacements.Container, seams []replacements.Seam) (shape, error) {
	out := shape{}
	for _, seam := range seams {
		for _, ref := range seam.Types {
			switch {
			case ref.Path == sqlDbPath && ref.Name == sqlDbName:
				out.Postgres = true
			case ref.Path == redisDbPath && ref.Name == redisDbName:
				out.Redis = true
			case ref.Path == loggerPath && ref.Name == loggerName:
				out.Logger = true
			case ref.Path == hiltonPath && ref.Name == hiltonName:
				out.Hilton = true
			case ref.Path == consumerPath && ref.Name == consumerName:
				out.Consumer = true
			case ref.Path == connPath && ref.Name == connName:
				out.Dead = append(out.Dead, conn{Field: seam.Field, Name: neighbour(seam.Field)})
			}
		}
	}
	slices.SortFunc(out.Dead, func(a, b conn) int { return strings.Compare(a.Field, b.Field) })

	file, err := parser.ParseFile(token.NewFileSet(), container.File, nil, parser.SkipObjectResolution)
	if err != nil {
		return shape{}, err
	}
	out.Grpc = commandArgs(file, "NewGrpcCommand")
	out.Http = commandArgs(file, "NewHttpCommand")
	return out, nil
}

// BaseGrpcClientConnInterfaceForContractProtoContractServiceClient → contract.
// Only a label for the panic message, so a miss costs nothing.
func neighbour(field string) string {
	_, after, found := strings.Cut(field, "For")
	if !found {
		after = field
	}
	for _, suffix := range []string{"ServiceClient", "Client", "Proto"} {
		after = strings.TrimSuffix(after, suffix)
	}
	if after == "" {
		return "neighbour"
	}
	// The first word is the neighbour: ContractProtoContract → contract.
	end := len(after)
	for i, r := range after {
		if i > 0 && unicode.IsUpper(r) {
			end = i
			break
		}
	}
	return strings.ToLower(after[:end])
}

// Read off the daemon's own construction call, so the harness copies the
// interceptor order rather than holding a second opinion about it.
func commandArgs(file *ast.File, constructor string) wiring {
	var out wiring
	body := initAppWithBody(file)
	if body == nil {
		return out
	}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name, ok := qualifiedBy(call.Fun, ""); !ok || name != constructor {
			return true
		}
		for _, arg := range call.Args {
			switch value := arg.(type) {
			case *ast.CompositeLit:
				out.assign(fields(value.Elts), typeName(value.Type))
			case *ast.SelectorExpr:
				out.single(field(value))
			}
		}
		return false
	})
	return out
}

func (w *wiring) assign(names []string, kind string) {
	switch {
	case len(names) == 0:
	case strings.Contains(kind, "StreamInterceptor"):
		w.Stream = names
	case strings.Contains(kind, "Interceptor"):
		w.Interceptors = names
	case strings.Contains(kind, "ServiceHandler"):
		w.Handlers = names
	}
}

// By suffix, not by position: the rest of NewHttpCommand's arguments are bare
// container fields too.
func (w *wiring) single(name string) {
	switch {
	case name == "":
	case strings.HasSuffix(name, "Mux"):
		w.Mux = name
	case strings.HasSuffix(name, "Router"):
		w.Router = name
	}
}

func fields(elts []ast.Expr) []string {
	out := make([]string, 0, len(elts))
	for _, elt := range elts {
		if name := field(elt); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// container.x reaches a test as the getter Application.X.
func field(expr ast.Expr) string {
	name, ok := qualifiedBy(expr, "container")
	if !ok {
		return ""
	}
	return exported(name)
}

func exported(name string) string {
	if name == "" {
		return ""
	}
	return string(unicode.ToUpper(rune(name[0]))) + name[1:]
}

func typeName(expr ast.Expr) string {
	if array, ok := expr.(*ast.ArrayType); ok {
		expr = array.Elt
	}
	if selector, ok := expr.(*ast.SelectorExpr); ok {
		return selector.Sel.Name
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
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

// An empty qualifier matches any package: the container imports the tool under
// whatever alias diogen picked.
func qualifiedBy(expr ast.Expr, ident string) (string, bool) {
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
