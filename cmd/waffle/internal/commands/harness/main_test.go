package harness

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	waffleHttp "gitlab.com/gorib/waffle/tools/http"
)

const grpcContainer = `package diogen

import (
	sql "gitlab.com/gorib/sql"
	pry "gitlab.com/gorib/pry"
	baseGrpc "google.golang.org/grpc"
	grpcCommand "gitlab.com/gorib/waffle/tools/grpc"
	server "gitlab.com/gorib/grpc/server"
)

type Replacements struct {
	SqlDb                                        sql.Db
	PryLogger                                    pry.Logger
	BaseGrpcClientConnInterfaceForContractProtoContractServiceClient baseGrpc.ClientConnInterface
}

// Diogen assigns every tagged collection to a field of its own and passes the
// field. A fixture that inlined the literals would test a container shape the
// generator does not produce.
type Application struct {
	sqlDb                                        sql.Db
	pryLogger                                    pry.Logger
	baseGrpcClientConnInterfaceForContractProtoContractServiceClient baseGrpc.ClientConnInterface
	grpcCommandServiceHandlerAsServiceTag        []grpcCommand.ServiceHandler
	serverInterceptorAsMiddlewareTag             []server.Interceptor
	serverStreamInterceptorAsStreamMiddlewareTag []server.StreamInterceptor
}

func InitAppWith(replacements Replacements) (*Application, error) {
	container.baseGrpcClientConnInterfaceForContractProtoContractServiceClient = replacements.BaseGrpcClientConnInterfaceForContractProtoContractServiceClient

	if container.baseGrpcClientConnInterfaceForContractProtoContractServiceClient == nil {
		s, err := grpcClient.New(env.NeedValue[string]("DUNGEON_HOST"), env.NeedValue[string]("DUNGEON_PORT"))
		if err != nil {
			return nil, err
		}
		container.baseGrpcClientConnInterfaceForContractProtoContractServiceClient = s
	}
	container.grpcCommandServiceHandlerAsServiceTag = []grpcCommand.ServiceHandler{container.shopGrpcOrderServiceAsGrpcCommandServiceHandler}
	container.serverInterceptorAsMiddlewareTag = []server.Interceptor{container.interceptorLogMWAsServerInterceptor}
	container.serverStreamInterceptorAsStreamMiddlewareTag = []server.StreamInterceptor{container.interceptorStreamLogMWAsServerStreamInterceptor}
	{
		s := grpcCommand.NewGrpcCommand(
			container.grpcCommandServiceHandlerAsServiceTag,
			container.serverInterceptorAsMiddlewareTag,
			container.serverStreamInterceptorAsStreamMiddlewareTag,
			container.appBaseCommandForGrpcCommandGrpcCommand,
		)
		container.grpcGrpcCommand = s
	}
	return container, nil
}
`

const httpContainer = `package diogen

import (
	pry "gitlab.com/gorib/pry"
	baseGrpc "google.golang.org/grpc"
	httpCommand "gitlab.com/gorib/waffle/tools/http"
)

type Replacements struct {
	PryLogger                                                    pry.Logger
	BaseGrpcClientConnInterfaceForMediaProtoMediaServiceClient   baseGrpc.ClientConnInterface
}

type Application struct {
	pryLogger                   pry.Logger
	httpServerMux               server.Mux
	httpServerRouterAsRouterTag []server.Router
}

func InitAppWith(replacements Replacements) (*Application, error) {
	container.httpServerRouterAsRouterTag = []server.Router{container.mediaHttpRouterAsServerRouter}
	{
		s := httpCommand.NewHttpCommand(container.httpServerMux, container.httpServerRouterAsRouterTag, container.pryLogger)
		container.httpHttpCommand = s
	}
	return container, nil
}
`

const singleRouterContainer = `package diogen

import (
	pry "gitlab.com/gorib/pry"
	httpCommand "gitlab.com/gorib/waffle/tools/http"
)

type Replacements struct {
	PryLogger pry.Logger
}

type Application struct {
	pryLogger        pry.Logger
	httpServerMux    server.Mux
	httpServerRouter server.Router
}

func InitAppWith(replacements Replacements) (*Application, error) {
	{
		s := httpCommand.NewHttpCommand(container.httpServerMux, container.httpServerRouter, container.pryLogger)
		container.httpHttpCommand = s
	}
	return container, nil
}
`

const bothContainer = `package diogen

import (
	sql "gitlab.com/gorib/sql"
	pry "gitlab.com/gorib/pry"
	grpcCommand "gitlab.com/gorib/waffle/tools/grpc"
	httpCommand "gitlab.com/gorib/waffle/tools/http"
	server "gitlab.com/gorib/grpc/server"
	httpServer "gitlab.com/gorib/http/server"
)

type Replacements struct {
	SqlDb     sql.Db
	PryLogger pry.Logger
}

type Application struct {
	sqlDb                                 sql.Db
	pryLogger                             pry.Logger
	grpcCommandServiceHandlerAsServiceTag []grpcCommand.ServiceHandler
	serverInterceptorAsMiddlewareTag      []server.Interceptor
	httpServerMux                         server.Mux
	httpServerRouterAsRouterTag           []httpServer.Router
}

func InitAppWith(replacements Replacements) (*Application, error) {
	container.grpcCommandServiceHandlerAsServiceTag = []grpcCommand.ServiceHandler{container.shopGrpcOrderServiceAsGrpcCommandServiceHandler}
	container.serverInterceptorAsMiddlewareTag = []server.Interceptor{container.interceptorLogMWAsServerInterceptor}
	{
		s := grpcCommand.NewGrpcCommand(
			container.grpcCommandServiceHandlerAsServiceTag,
			container.serverInterceptorAsMiddlewareTag,
			nil,
			container.appBaseCommandForGrpcCommandGrpcCommand,
		)
		container.grpcGrpcCommand = s
	}
	{
		s := httpCommand.NewHttpCommand(container.httpServerMux, container.httpServerRouterAsRouterTag, container.pryLogger)
		container.httpHttpCommand = s
	}
	return container, nil
}
`

const consumerContainer = `package diogen

import (
	pry "gitlab.com/gorib/pry"
	scribus "gitlab.com/gorib/waffle/tools/scribus"
	grpcCommand "gitlab.com/gorib/waffle/tools/grpc"
	server "gitlab.com/gorib/grpc/server"
)

type Replacements struct {
	PryLogger       pry.Logger
	ScribusConsumer scribus.Consumer
}

type Application struct {
	pryLogger                             pry.Logger
	scribusConsumer                       scribus.Consumer
	grpcCommandServiceHandlerAsServiceTag []grpcCommand.ServiceHandler
	serverInterceptorAsMiddlewareTag      []server.Interceptor
}

func InitAppWith(replacements Replacements) (*Application, error) {
	container.grpcCommandServiceHandlerAsServiceTag = []grpcCommand.ServiceHandler{container.shopGrpcOrderServiceAsGrpcCommandServiceHandler}
	container.serverInterceptorAsMiddlewareTag = []server.Interceptor{container.interceptorLogMWAsServerInterceptor}
	{
		s := grpcCommand.NewGrpcCommand(
			container.grpcCommandServiceHandlerAsServiceTag,
			container.serverInterceptorAsMiddlewareTag,
			nil,
			container.appBaseCommandForGrpcCommandGrpcCommand,
		)
		container.grpcGrpcCommand = s
	}
	return container, nil
}
`

const cacheContainer = `package diogen

import (
	sql "gitlab.com/gorib/sql"
	redis "gitlab.com/gorib/redis"
	hilton "gitlab.com/gorib/hilton"
	pry "gitlab.com/gorib/pry"
	grpcCommand "gitlab.com/gorib/waffle/tools/grpc"
	server "gitlab.com/gorib/grpc/server"
)

type Replacements struct {
	SqlDb          sql.Db
	RedisDb        redis.Db
	HiltonProvider hilton.Provider
	PryLogger      pry.Logger
}

func InitAppWith(replacements Replacements) (*Application, error) {
	{
		s := grpcCommand.NewGrpcCommand(
			[]grpcCommand.ServiceHandler{container.shopGrpcOrderServiceAsGrpcCommandServiceHandler},
			[]server.Interceptor{container.interceptorLogMWAsServerInterceptor},
			nil,
			container.appBaseCommandForGrpcCommandGrpcCommand,
		)
		container.grpcGrpcCommand = s
	}
	return container, nil
}
`

func module(t *testing.T, container string, extra map[string]string) {
	t.Helper()
	t.Chdir(t.TempDir())

	files := map[string]string{
		"go.mod": "module example.test/shop\n\ngo 1.27\n",
		"config/di/di.go": "//go:generate diogen -output config/diogen/di_gen.go -pkg diogen $GOFILE\n\n" +
			"package di\n",
		"config/diogen/di_gen.go": container,
	}
	for name, body := range extra {
		files[name] = body
	}
	for name, body := range files {
		path := filepath.FromSlash(name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func written(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.FromSlash(name))
	if err != nil {
		t.Fatalf("%s was not written: %v", name, err)
	}
	return string(body)
}

func TestBareRunWritesNothing(t *testing.T) {
	module(t, grpcContainer, nil)

	if err := New().Run(t.Context()); err != nil {
		t.Fatalf("bare run: %v", err)
	}

	if _, err := os.Stat(filepath.FromSlash("tests/internal/harness/resources.go")); err == nil {
		t.Fatal("a bare run wrote the harness")
	}
}

func TestInitWritesTheHarnessForAServiceWithADatabase(t *testing.T) {
	module(t, grpcContainer, map[string]string{
		"domain/order/cmd/migrate/main.go": "package migrate\n",
	})

	if err := New().Run(t.Context(), "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	resources := written(t, "tests/internal/harness/resources.go")
	for _, want := range []string{
		`sqltest.Env("SHOP_TEST_DSN")`,
		`sqltest.Database("shop_test")`,
		"sqltest.Ephemeral()",
		"sqltest.Migrations(waffleMigrate.Up(",
		"orderMigrate.NewMigrations()",
		"resource.New(postgres)",
	} {
		if !strings.Contains(resources, want) {
			t.Errorf("resources.go is missing %q:\n%s", want, resources)
		}
	}
	// The loop belongs to the migrate command, so a harness that spells goose
	// out again is one that can build a schema the daemon does not.
	if strings.Contains(resources, "goose.") {
		t.Errorf("resources.go drives goose itself:\n%s", resources)
	}

	boot := written(t, "tests/internal/harness/boot.go")
	for _, want := range []string{
		"cut.SqlDb = postgres.Db()",
		"cut.PryLogger = logger",
		`grpcClient.Dead("contract")`,
		// Boot brings the resources up itself, so a scenario cannot forget.
		"if err := Up(ctx); err != nil {",
	} {
		if !strings.Contains(boot, want) {
			t.Errorf("boot.go is missing %q:\n%s", want, boot)
		}
	}
}

// A recorder the harness installs is not a seam the scenario has to be granted:
// tests/internal is outside the allow-list scan, and the handle comes back on
// App instead of going in through Replacements.
func TestInitReturnsRecordersOnApp(t *testing.T) {
	module(t, consumerContainer, nil)

	if err := New().Run(t.Context(), "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	boot := written(t, "tests/internal/harness/boot.go")
	for _, want := range []string{
		"*scribus.ConsumerRecorder",
		"*pry.LogRecorder",
		"consumer := scribus.Recorder()",
		"logger := pry.Recorder()",
		"cut.ScribusConsumer = consumer",
		"Consumer: consumer",
		"Logger: logger",
	} {
		if !strings.Contains(boot, want) {
			t.Errorf("boot.go is missing %q:\n%s", want, boot)
		}
	}
}

func TestInitWritesAGrpcServer(t *testing.T) {
	module(t, grpcContainer, nil)

	if err := New().Run(t.Context(), "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	body := written(t, "tests/internal/harness/serve.go")
	for _, want := range []string{
		"waffleGrpc.Serve(",
		"a.GrpcCommandServiceHandlerAsServiceTag()",
		"a.ServerInterceptorAsMiddlewareTag()",
		"a.ServerStreamInterceptorAsStreamMiddlewareTag()",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("serve.go is missing %q:\n%s", want, body)
		}
	}
	// A member by name means the list was copied, and a tagged registration
	// added later would reach the daemon and not the suite.
	if strings.Contains(body, "a.InterceptorLogMWAsServerInterceptor()") {
		t.Errorf("the interceptor list was expanded instead of read whole:\n%s", body)
	}
	if strings.Contains(body, "waffleHttp") {
		t.Errorf("a service that serves no HTTP got an HTTP server:\n%s", body)
	}
}

// Two of the nine services in sherpa serve both, and two files could not
// coexist: each would declare Clients and Serve in one package.
func TestInitWritesBothTransportsIntoOneFile(t *testing.T) {
	module(t, bothContainer, nil)

	if err := New().Run(t.Context(), "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	body := written(t, "tests/internal/harness/serve.go")
	for _, want := range []string{
		"waffleGrpc.Serve(",
		"a.GrpcCommandServiceHandlerAsServiceTag()",
		"waffleHttp.Serve(",
		"a.HttpServerMux()",
		"a.HttpServerRouterAsRouterTag()",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("serve.go is missing %q:\n%s", want, body)
		}
	}
	if got := strings.Count(body, "type Clients struct"); got != 1 {
		t.Errorf("Clients is declared %d times:\n%s", got, body)
	}
}

// A gateway: no database at all, and the caller reaches it over HTTP.
func TestInitWritesAnHttpServerAndAnEmptyResourceSet(t *testing.T) {
	module(t, httpContainer, nil)

	if err := New().Run(t.Context(), "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	resources := written(t, "tests/internal/harness/resources.go")
	if !strings.Contains(resources, "resource.New()") {
		t.Errorf("a service with no database did not get an empty set:\n%s", resources)
	}
	for _, unwanted := range []string{"sqltest", "postgres"} {
		if strings.Contains(resources, unwanted) {
			t.Errorf("a service with no database got %q:\n%s", unwanted, resources)
		}
	}
	if boot := written(t, "tests/internal/harness/boot.go"); strings.Contains(boot, "cut.SqlDb") {
		t.Errorf("a service with no database got a database in the cut:\n%s", boot)
	}

	server := written(t, "tests/internal/harness/serve.go")
	for _, want := range []string{"waffleHttp.Serve(", "a.HttpServerMux()", "a.HttpServerRouterAsRouterTag()"} {
		if !strings.Contains(server, want) {
			t.Errorf("serve.go is missing %q:\n%s", want, server)
		}
	}
	if strings.Contains(server, "waffleGrpc") {
		t.Errorf("a service that serves no gRPC got a gRPC server:\n%s", server)
	}
}

// Nothing here compiles what it scaffolds, so the call is read against the
// real signature instead.
func TestInitWritesAServeCallThatFitsTheHelper(t *testing.T) {
	module(t, httpContainer, nil)

	if err := New().Run(t.Context(), "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	call, bound := serveCall(t, "tests/internal/harness/serve.go", "waffleHttp")
	signature := reflect.TypeOf(waffleHttp.Serve)
	if len(call.Args) != signature.NumIn() {
		t.Errorf("serve.go passes %d arguments, Serve takes %d", len(call.Args), signature.NumIn())
	}
	if bound != signature.NumOut() {
		t.Errorf("serve.go binds %d results, Serve returns %d", bound, signature.NumOut())
	}
}

func serveCall(t *testing.T, name, pkg string) (call *ast.CallExpr, bound int) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filepath.FromSlash(name), nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("serve.go does not parse: %v", err)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 {
			return true
		}
		found, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := found.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Serve" {
			return true
		}
		if x, ok := selector.X.(*ast.Ident); !ok || x.Name != pkg {
			return true
		}
		call, bound = found, len(assign.Lhs)
		return false
	})
	if call == nil {
		t.Fatalf("serve.go has no %s.Serve call", pkg)
	}
	return call, bound
}

// The alternative is silent: no router recognised reads as a service serving
// no HTTP at all, and the suite loses its client.
func TestInitRefusesAContainerWiredToASingleRouter(t *testing.T) {
	module(t, singleRouterContainer, nil)

	err := New().Run(t.Context(), "init")

	if err == nil {
		t.Fatal("a container that predates the collection was accepted")
	}
	for _, want := range []string{"NewHttpCommand", "RouterTag"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %s: %v", want, err)
		}
	}
}

// A second system is a second entry in the one registry, and the order of the
// entries is the order they are declared above it.
func TestInitWritesRedisBesideTheDatabase(t *testing.T) {
	module(t, cacheContainer, nil)

	if err := New().Run(t.Context(), "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	resources := written(t, "tests/internal/harness/resources.go")
	for _, want := range []string{
		`redistest.Env("SHOP_TEST_REDIS")`,
		`redistest.Docker("redis:8-alpine")`,
		"resource.New(postgres, cache)",
	} {
		if !strings.Contains(resources, want) {
			t.Errorf("resources.go is missing %q:\n%s", want, resources)
		}
	}

	// The seam the container builds eagerly out of REDIS_HOST when nobody fills it.
	if boot := written(t, "tests/internal/harness/boot.go"); !strings.Contains(boot, "cut.RedisDb = cache.Db()") {
		t.Errorf("boot.go leaves redis to the environment:\n%s", boot)
	}
}

// The container's own default is hilton.Noop(), so nothing breaks without this
// — the span a scenario wants to read back merely does not exist.
func TestInitRecordsTelemetryRatherThanSilencingIt(t *testing.T) {
	module(t, cacheContainer, nil)

	if err := New().Run(t.Context(), "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	boot := written(t, "tests/internal/harness/boot.go")
	for _, want := range []string{
		"Telemetry *hilton.Recording",
		"telemetry := hilton.Recorder()",
		"cut.HiltonProvider = telemetry",
		"Telemetry: telemetry",
	} {
		if !strings.Contains(boot, want) {
			t.Errorf("boot.go is missing %q:\n%s", want, boot)
		}
	}
	// The assignment, not the word: the comment above it names Noop as the thing
	// being rejected.
	if strings.Contains(boot, "= hilton.Noop()") {
		t.Errorf("the harness silenced telemetry instead of recording it:\n%s", boot)
	}
}

func TestInitLeavesRedisOutWhenTheContainerHasNone(t *testing.T) {
	module(t, grpcContainer, nil)

	if err := New().Run(t.Context(), "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	resources := written(t, "tests/internal/harness/resources.go")
	for _, unwanted := range []string{"redistest", "cache"} {
		if strings.Contains(resources, unwanted) {
			t.Errorf("a service with no redis got %q:\n%s", unwanted, resources)
		}
	}
}

func TestInitWritesTheSuiteAndTheChecks(t *testing.T) {
	module(t, grpcContainer, nil)

	if err := New().Run(t.Context(), "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	if body := written(t, "tests/suite_test.go"); !strings.Contains(body, "harness.Down(") {
		t.Errorf("main_test.go does not tear the resources down:\n%s", body)
	}
	if body := written(t, "tests/boot_test.go"); !strings.Contains(body, "harness.Boot(") {
		t.Errorf("boot_test.go does not boot:\n%s", body)
	}
	if body := written(t, "config/diogen/replacements_test.go"); !strings.Contains(body, "replacements.Check()") {
		t.Errorf("the allow-list test is missing:\n%s", body)
	}
	if body := written(t, "config/diogen/boundaries_test.go"); !strings.Contains(body, "boundaries.Check()") {
		t.Errorf("the domain-boundary test is missing:\n%s", body)
	}
}

func TestInitLeavesAnExistingFileAlone(t *testing.T) {
	module(t, grpcContainer, map[string]string{
		"tests/internal/harness/resources.go": "package harness // mine\n",
	})

	if err := New().Run(t.Context(), "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	if body := written(t, "tests/internal/harness/resources.go"); !strings.Contains(body, "// mine") {
		t.Fatal("init overwrote a harness somebody had already written")
	}
}

func TestUnknownSubcommandIsRefused(t *testing.T) {
	module(t, grpcContainer, nil)

	err := New().Run(t.Context(), "generate")

	if err == nil || !strings.Contains(err.Error(), "generate") {
		t.Fatalf("run: %v", err)
	}
}
