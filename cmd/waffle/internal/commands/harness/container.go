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

// Each field is one container getter, not a list of members: the collection is
// a container field of its own, so the harness reads it whole and a tagged
// registration added later reaches the suite without an edit here.
type wiring struct {
	Handlers     string
	Interceptors string
	Stream       string
	Mux          string
	Router       string
}

func (w wiring) Present() bool { return w.Handlers != "" || w.Mux != "" }

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
	types := containerFields(file)
	out.Grpc = commandArgs(file, types, "NewGrpcCommand")
	out.Http = commandArgs(file, types, "NewHttpCommand")
	return out, nil
}

// Field name to the name of its type, off the Application struct. Arguments to
// the daemon's constructors are bare container fields, so this is what says
// which of them is the interceptor chain and which is the handler list.
func containerFields(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, decl := range file.Decls {
		generic, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range generic.Specs {
			typed, ok := spec.(*ast.TypeSpec)
			if !ok || typed.Name.Name != "Application" {
				continue
			}
			structure, ok := typed.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					out[name.Name] = typeName(field.Type)
				}
			}
		}
	}
	return out
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

// Read off the daemon's own construction call, so the harness serves what the
// daemon serves rather than holding a second opinion about it.
func commandArgs(file *ast.File, types map[string]string, constructor string) wiring {
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
			if name, ok := qualifiedBy(arg, "container"); ok {
				out.assign(name, types[name])
			}
		}
		return false
	})
	return out
}

// By the field's own type, not by position: every argument here is a bare
// container field, and the constructors take more of them than these.
func (w *wiring) assign(name, kind string) {
	getter := exported(name)
	switch {
	case getter == "" || kind == "":
	case strings.Contains(kind, "StreamInterceptor"):
		w.Stream = getter
	case strings.Contains(kind, "Interceptor"):
		w.Interceptors = getter
	case strings.Contains(kind, "ServiceHandler"):
		w.Handlers = getter
	case strings.HasSuffix(kind, "Mux"):
		w.Mux = getter
	case strings.HasSuffix(kind, "Router"):
		w.Router = getter
	}
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
