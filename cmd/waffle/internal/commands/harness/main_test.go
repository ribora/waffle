package harness

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	waffleHttp "gitlab.com/gorib/waffle/tools/http"
)

const grpcContainer = `package diogen

import (
	"time"

	env "gitlab.com/gorib/env"
	grpcClient "gitlab.com/gorib/grpc/client"
	server "gitlab.com/gorib/grpc/server"
	pry "gitlab.com/gorib/pry"
	sql "gitlab.com/gorib/sql"
	grpcCommand "gitlab.com/gorib/waffle/tools/grpc"
	contractProto "example.test/api/contract"
	shopGrpc "example.test/shop/domain/shop/cmd/grpc"
	baseGrpc "google.golang.org/grpc"
)

type Replacements struct {
	SqlDb                                                            sql.Db
	PryLogger                                                        pry.Logger
	BaseGrpcClientConnInterfaceForContractProtoContractServiceClient baseGrpc.ClientConnInterface
	ContractProtoContractServiceClient                               contractProto.ContractServiceClient
	ShopThing                                                        shopGrpc.Thing
	GrpcCommandServiceHandlerAsServiceTag                            []grpcCommand.ServiceHandler
}

// Diogen assigns every tagged collection to a field of its own and passes the
// field. A fixture that inlined the literals would test a container shape the
// generator does not produce.
type Application struct {
	sqlDb                                                            sql.Db
	pryLogger                                                        pry.Logger
	baseGrpcClientConnInterfaceForContractProtoContractServiceClient baseGrpc.ClientConnInterface
	contractProtoContractServiceClient                               contractProto.ContractServiceClient
	shopThing                                                        shopGrpc.Thing
	grpcCommandServiceHandlerAsServiceTag                            []grpcCommand.ServiceHandler
	serverInterceptorAsMiddlewareTag                                 []server.Interceptor
	serverStreamInterceptorAsStreamMiddlewareTag                     []server.StreamInterceptor
}

func InitAppWith(replacements Replacements) (*Application, error) {
	container.sqlDb = replacements.SqlDb

	if container.sqlDb == nil {
		s, err := sql.NewDb(env.NeedValue[string]("DB_DRIVER"), env.NeedValue[string]("DB_DSN"))
		if err != nil {
			return nil, err
		}
		container.sqlDb = s
	}

	container.baseGrpcClientConnInterfaceForContractProtoContractServiceClient = replacements.BaseGrpcClientConnInterfaceForContractProtoContractServiceClient

	if container.baseGrpcClientConnInterfaceForContractProtoContractServiceClient == nil {
		s, err := grpcClient.New(env.NeedValue[string]("DUNGEON_HOST"), env.NeedValue[string]("DUNGEON_PORT"))
		if err != nil {
			return nil, err
		}
		container.baseGrpcClientConnInterfaceForContractProtoContractServiceClient = s
	}

	container.contractProtoContractServiceClient = replacements.ContractProtoContractServiceClient

	if container.contractProtoContractServiceClient == nil {
		s := contractProto.NewContractServiceClient(container.baseGrpcClientConnInterfaceForContractProtoContractServiceClient)
		container.contractProtoContractServiceClient = s
	}

	container.shopThing = replacements.ShopThing

	if container.shopThing == nil {
		s := shopGrpc.NewThing(env.NeedValue[string]("THING_URL"), env.NeedValue[uint16]("THING_PORT"), env.NeedValue[time.Duration]("THING_TTL"), env.NeedJson[shopGrpc.Config]("THING_CONFIG"),
			env.NeedArray[int]("THING_SHARDS"), env.Value("LOGLEVEL", "info"), env.Value("POLL", 5*time.Second), env.Array("ORIGINS", []string{}),
			env.Value("LOCAL", shopGrpc.Fallback), env.Value("SQL_DSN", "x"), env.NeedValue[string]("Custom"))
		container.shopThing = s
	}
	if container.sqlDb == nil {
		_ = env.Value("SQL_DSN", "x")
	}

	container.grpcCommandServiceHandlerAsServiceTag = replacements.GrpcCommandServiceHandlerAsServiceTag

	if container.grpcCommandServiceHandlerAsServiceTag == nil {
		s := shopGrpc.NewOrderService(container.sqlDb)
		container.shopGrpcOrderServiceAsGrpcCommandServiceHandler = s
	}
	if container.grpcCommandServiceHandlerAsServiceTag == nil {
		container.grpcCommandServiceHandlerAsServiceTag = []grpcCommand.ServiceHandler{container.shopGrpcOrderServiceAsGrpcCommandServiceHandler}
	}
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

const orderHandler = `package grpc

import (
	"google.golang.org/grpc"

	proto "example.test/api/order"
)

func NewOrderService(db any) *orderService {
	return &orderService{}
}

type orderService struct {
	proto.OrderServiceServer
}

func (s *orderService) Start(server grpc.ServiceRegistrar) {
	proto.RegisterOrderServiceServer(server, s)
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
	httpCommandGlobal           httpCommand.Global
	httpServerRouterAsRouterTag []server.Router
}

func InitAppWith(replacements Replacements) (*Application, error) {
	container.httpServerRouterAsRouterTag = []server.Router{container.mediaHttpRouterAsServerRouter}
	{
		s := httpCommand.NewHttpCommand(container.httpServerMux, container.httpCommandGlobal, container.httpServerRouterAsRouterTag, container.pryLogger)
		container.httpHttpCommand = s
	}
	return container, nil
}
`

const noGlobalContainer = `package diogen

import (
	pry "gitlab.com/gorib/pry"
	httpCommand "gitlab.com/gorib/waffle/tools/http"
)

type Replacements struct {
	PryLogger pry.Logger
}

type Application struct {
	pryLogger                   pry.Logger
	httpServerMux               server.Mux
	httpServerRouterAsRouterTag []server.Router
}

func InitAppWith(replacements Replacements) (*Application, error) {
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
	httpCommandGlobal                     httpCommand.Global
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
		s := httpCommand.NewHttpCommand(container.httpServerMux, container.httpCommandGlobal, container.httpServerRouterAsRouterTag, container.pryLogger)
		container.httpHttpCommand = s
	}
	return container, nil
}
`

const recorderContainer = `package diogen

import (
	amqp "gitlab.com/gorib/amqp"
	pry "gitlab.com/gorib/pry"
	scribus "gitlab.com/gorib/waffle/tools/scribus"
)

type Replacements struct {
	PryLogger       pry.Logger
	ScribusConsumer scribus.Consumer
	AmqpPublisher   amqp.Publisher
	PryLoggers      []pry.Logger
}

type Application struct{}

func InitAppWith(replacements Replacements) (*Application, error) {
	return container, nil
}
`

const cacheContainer = `package diogen

import (
	sql "gitlab.com/gorib/sql"
	redis "gitlab.com/gorib/redis"
	hilton "gitlab.com/gorib/hilton"
	pry "gitlab.com/gorib/pry"
)

type Replacements struct {
	SqlDb          sql.Db
	RedisDb        redis.Db
	HiltonProvider hilton.Provider
	PryLogger      pry.Logger
}

type Application struct{}

func InitAppWith(replacements Replacements) (*Application, error) {
	return container, nil
}
`

const paramsContainer = `package diogen

import (
	"context"

	di "example.test/shop/config/di"
	client "example.test/shop/client"
	pry "gitlab.com/gorib/pry"
)

type Limits struct{}

type Replacements struct {
	PryLogger pry.Logger
}

type Application struct{}

func InitAppWith(ctx context.Context, app *di.App, clientOpts *client.Options, limits Limits, onClose func(name string) error, replacements Replacements) (*Application, error) {
	return container, nil
}
`

const contextContainer = `package diogen

import "context"

type Replacements struct{}

type Application struct{}

func InitAppWith(ctx context.Context, replacements Replacements) (*Application, error) {
	return container, nil
}
`

const paramsHook = "package harness\n\n" +
	"func (a *App) params(ctx context.Context) (app *di.App, clientOpts *client.Options, limits diogen.Limits, onClose func(string) error, err error) {\n" +
	"\treturn nil, nil, diogen.Limits{}, nil, nil\n}\n"

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
		place(t, name, body)
	}
}

func place(t *testing.T, name, body string) {
	t.Helper()
	path := filepath.FromSlash(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, args ...string) {
	t.Helper()
	if err := New().Run(t.Context(), args...); err != nil {
		t.Fatalf("run: %v", err)
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

func generatedFile(t *testing.T) string {
	t.Helper()
	return written(t, "tests/internal/harness/harness_gen.go")
}

func contains(t *testing.T, name, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("%s is missing %q:\n%s", name, want, body)
		}
	}
}

func lacks(t *testing.T, name, body string, unwanted ...string) {
	t.Helper()
	for _, bad := range unwanted {
		if strings.Contains(body, bad) {
			t.Errorf("%s holds %q:\n%s", name, bad, body)
		}
	}
}

func TestFirstRunScaffoldsTheSuiteAndGeneratesTheHarness(t *testing.T) {
	module(t, grpcContainer, map[string]string{
		"domain/order/cmd/migrate/main.go": "package migrate\n",
	})

	run(t)

	contains(t, "harness.go", written(t, "tests/internal/harness/harness.go"), "//go:generate go tool waffle tests:harness")
	contains(t, "suite_test.go", written(t, "tests/suite_test.go"), "harness.Main(m)")
	contains(t, "boot_test.go", written(t, "tests/boot_test.go"), "harness.Boot(", "app.Serve()")
	contains(t, "migrations_test.go", written(t, "tests/migrations_test.go"), "harness.CheckMigrations(")
	contains(t, "replacements_test.go", written(t, "config/diogen/replacements_test.go"), "replacements.Check()")
	contains(t, "boundaries_test.go", written(t, "config/diogen/boundaries_test.go"), "boundaries.Check()")

	contains(t, "harness_gen.go", generatedFile(t),
		"// Code generated by waffle tests:harness. DO NOT EDIT.",
		`sqltest.Env("SHOP_TEST_DSN")`,
		`sqltest.Database("shop_test")`,
		"sqltest.Ephemeral()",
		"orderMigrate.NewMigrations()",
		"resource.New(postgres",
		// Boot brings the resources up itself, so a scenario cannot forget.
		"if err := Up(ctx); err != nil {",
		"if cut.SqlDb == nil {\n\t\tcut.SqlDb = postgres.Db()",
	)
}

// The loop belongs to the migrate command, so a harness that spells goose out
// again is one that can build a schema the daemon does not.
func TestMigrationsGoThroughTheCommandsOwnLoopInNameOrder(t *testing.T) {
	module(t, grpcContainer, map[string]string{
		"domain/rent/cmd/migrate/main.go":    "package migrate\n",
		"domain/invoice/cmd/migrate/main.go": "package migrate\n",
	})

	run(t)

	body := generatedFile(t)
	lacks(t, "harness_gen.go", body, "goose.")
	if strings.Index(body, "invoiceMigrate.NewMigrations(),\n\trentMigrate") < 0 {
		t.Errorf("the sources are not in name order:\n%s", body)
	}
	contains(t, "harness_gen.go", body,
		"waffleMigrate.CheckShared(sources)",
		`"invoice": invoiceMigrate.NewMigrations()`,
		"sqltest.Suffix(name)",
	)
}

// Only the connection's label, and read off the container rather than off a
// naming convention for environment variables.
func TestDeadNamesTheClientsOnTheConnectionAndWhatItReads(t *testing.T) {
	module(t, grpcContainer, nil)

	run(t)

	contains(t, "harness_gen.go", generatedFile(t),
		"if cut.BaseGrpcClientConnInterfaceForContractProtoContractServiceClient == nil {",
		`grpcClient.Dead("contract.ContractServiceClient (DUNGEON_HOST, DUNGEON_PORT)")`,
	)
}

// A field per variable, typed as the read is, so a misspelt key or a wrong type
// in the env hook does not compile.
func TestVariablesHoldWhatAnUnreplacedConstructorReads(t *testing.T) {
	module(t, grpcContainer, nil)

	run(t)

	body := generatedFile(t)
	for _, field := range []string{
		"THING_URL string", "THING_PORT uint16", "THING_TTL time.Duration", "THING_CONFIG string", "THING_SHARDS []int",
	} {
		name, typ, _ := strings.Cut(field, " ")
		if !regexp.MustCompile(`(?m)^\t` + name + `\s+` + regexp.QuoteMeta(typ) + `$`).MatchString(body) {
			t.Errorf("Variables has no %s:\n%s", field, body)
		}
	}
	contains(t, "harness_gen.go", body,
		`"thing-url.harness.invalid"`,
		`"null"`,
		`put("THING_PORT", v.THING_PORT)`,
		`put("THING_SHARDS", list(v.THING_SHARDS))`,
		`"time"`,
	)
	// Under a seam Boot fills, so the constructor never runs.
	lacks(t, "harness_gen.go", body, "\tDB_DSN ", "\tDUNGEON_HOST ")
}

// The fallback is the container's answer, so it is the field's default and is
// written only when the hook changed it: [] does not survive the environment.
func TestAFallbackIsSetOnlyWhenTheHookChangesIt(t *testing.T) {
	module(t, grpcContainer, nil)

	run(t)

	body := generatedFile(t)
	contains(t, "harness_gen.go", body,
		`LOGLEVEL:`, `"info",`,
		"5 * time.Second",
		"[]string{}",
		"if v.LOGLEVEL != d.LOGLEVEL {",
		"if !slices.Equal(v.ORIGINS, d.ORIGINS) {",
		// Read under a filled seam and under an unfilled one: the second decides.
		"SQL_DSN",
	)
	// A fallback naming a declaration of its own file cannot be copied out of it.
	lacks(t, "harness_gen.go", body, "LOCAL")
}

// The way out for a key no code reads through env, and not a way around a field:
// a key that has one is refused.
func TestCustomTakesWhatHasNoFieldAndRefusesWhatHasOne(t *testing.T) {
	module(t, grpcContainer, nil)

	run(t)

	body := generatedFile(t)
	contains(t, "harness_gen.go", body,
		"Custom map[string]string",
		`"THING_URL": true`,
		"for key, value := range v.Custom {",
		"if typed[key] {",
	)
	if got := strings.Count(body, "\tCustom "); got != 1 {
		t.Errorf("Custom is declared %d times — a variable of that name took the field:\n%s", got, body)
	}
}

func TestAVariableOnlyTheModuleReadsIsAFieldToo(t *testing.T) {
	module(t, grpcContainer, map[string]string{
		"domain/shop/adapter/client.go": "package adapter\n\nimport \"gitlab.com/gorib/env\"\n\n" +
			"var key = env.NeedValue[string](\"SHOP_KEY\")\n\nvar skip = env.NeedValue[string](\"THING_URL\")\n",
	})

	run(t)

	body := generatedFile(t)
	contains(t, "harness_gen.go", body, `"shop-key.harness.invalid"`)
	if got := strings.Count(body, "\tTHING_URL "); got != 1 {
		t.Errorf("THING_URL is declared %d times:\n%s", got, body)
	}
}

// A recorder the harness installs is not a seam the scenario has to be granted:
// tests/internal is outside the allow-list scan, and the handle comes back on
// App instead of going in through Replacements.
func TestRecordersComeBackOnApp(t *testing.T) {
	module(t, recorderContainer, nil)

	run(t)

	body := generatedFile(t)
	contains(t, "harness_gen.go", body,
		"Logger    *pry.LogRecorder",
		"Consumer  *scribus.ConsumerRecorder",
		"Publisher *amqp.PublisherRecorder",
		"cut.PryLogger, app.Logger = logger, logger",
		"cut.ScribusConsumer, app.Consumer = consumer, consumer",
		"cut.AmqpPublisher, app.Publisher = publisher, publisher",
	)
	// A collection of loggers is a di.Tags seam, and one recorder is not a list.
	lacks(t, "harness_gen.go", body, "cut.PryLoggers")
}

func TestRedisBesideTheDatabaseAndTelemetryRecorded(t *testing.T) {
	module(t, cacheContainer, nil)

	run(t)

	body := generatedFile(t)
	contains(t, "harness_gen.go", body,
		`redistest.Env("SHOP_TEST_REDIS")`,
		`redistest.Docker("redis:8-alpine")`,
		"resource.New(postgres, cache)",
		"cut.RedisDb = cache.Db()",
		"Telemetry *hilton.Recording",
		"cut.HiltonProvider, app.Telemetry = telemetry, telemetry",
	)
	lacks(t, "harness_gen.go", body, "= hilton.Noop()")
}

func TestRedisIsLeftOutWhenTheContainerHasNone(t *testing.T) {
	module(t, grpcContainer, nil)

	run(t)

	lacks(t, "harness_gen.go", generatedFile(t), "redistest", "cache")
}

func TestAGrpcServiceGetsATypedClientPerRegisteredHandler(t *testing.T) {
	module(t, grpcContainer, map[string]string{
		"domain/shop/cmd/grpc/service.go": orderHandler,
	})

	run(t)

	body := generatedFile(t)
	contains(t, "harness_gen.go", body,
		`orderProto "example.test/api/order"`,
		"Order orderProto.OrderServiceClient",
		"clients.Order = orderProto.NewOrderServiceClient(conn)",
		"waffleGrpc.Serve(",
		"a.GrpcCommandServiceHandlerAsServiceTag()",
		"a.ServerInterceptorAsMiddlewareTag()",
		"a.ServerStreamInterceptorAsStreamMiddlewareTag()",
	)
	// A member by name means the list was copied, and a tagged registration
	// added later would reach the daemon and not the suite.
	lacks(t, "harness_gen.go", body, "a.InterceptorLogMWAsServerInterceptor()", "waffleHttp")
}

func TestServicesOfOneNameArePrefixedByTheirPackage(t *testing.T) {
	clients := []client{
		{Path: "example.test/api/notification", Service: "TemplateService"},
		{Path: "example.test/api/document", Service: "TemplateService"},
		{Path: "example.test/api/document", Service: "DocumentService"},
	}

	name(clients)

	got := map[string]string{}
	for _, c := range clients {
		got[c.Field] = c.Alias
	}
	want := map[string]string{
		"NotificationTemplate": "notificationProto",
		"DocumentTemplate":     "documentProto",
		"Document":             "documentProto",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields %v, want %v", got, want)
	}
}

// Two of the nine services in sherpa serve both, and one Clients has to carry
// both transports.
func TestBothTransportsShareOneClients(t *testing.T) {
	module(t, bothContainer, nil)

	run(t)

	body := generatedFile(t)
	contains(t, "harness_gen.go", body,
		"waffleGrpc.Serve(",
		"waffleHttp.Serve(",
		"a.HttpServerMux()",
		"a.HttpCommandGlobal()",
		"a.HttpServerRouterAsRouterTag()",
	)
	if got := strings.Count(body, "type Clients struct"); got != 1 {
		t.Errorf("Clients is declared %d times:\n%s", got, body)
	}
}

// A gateway: no database at all, and the caller reaches it over HTTP.
func TestAGatewayGetsAnEmptyResourceSetAndAnHttpServer(t *testing.T) {
	module(t, httpContainer, nil)

	run(t)

	body := generatedFile(t)
	contains(t, "harness_gen.go", body, "resource.New()", "waffleHttp.Serve(")
	lacks(t, "harness_gen.go", body, "sqltest", "postgres", "cut.SqlDb", "waffleGrpc", "CheckMigrations")
	if _, err := os.Stat(filepath.FromSlash("tests/migrations_test.go")); err == nil {
		t.Error("a service with no database got a migrations test")
	}
}

// Nothing here compiles what it generates, so the call is read against the real
// signature instead.
func TestTheServeCallFitsTheHelper(t *testing.T) {
	module(t, httpContainer, nil)

	run(t)

	call, bound := serveCall(t, "tests/internal/harness/harness_gen.go", "waffleHttp")
	signature := reflect.TypeOf(waffleHttp.Serve)
	if len(call.Args) != signature.NumIn() {
		t.Errorf("Serve is passed %d arguments, and takes %d", len(call.Args), signature.NumIn())
	}
	if bound != signature.NumOut() {
		t.Errorf("Serve's results are bound to %d names, and it returns %d", bound, signature.NumOut())
	}
}

func serveCall(t *testing.T, name, pkg string) (call *ast.CallExpr, bound int) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filepath.FromSlash(name), nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("%s does not parse: %v", name, err)
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
		t.Fatalf("%s has no %s.Serve call", name, pkg)
	}
	return call, bound
}

// The alternative is silent: no router recognised reads as a service serving
// no HTTP at all, and the suite loses its client.
func TestAContainerWiredToASingleRouterIsRefused(t *testing.T) {
	module(t, singleRouterContainer, nil)

	err := New().Run(t.Context())

	if err == nil {
		t.Fatal("a container that predates the collection was accepted")
	}
	for _, want := range []string{"NewHttpCommand", "RouterTag"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %s: %v", want, err)
		}
	}
}

func TestAContainerWithNoGlobalChainIsRefused(t *testing.T) {
	module(t, noGlobalContainer, nil)

	err := New().Run(t.Context())

	if err == nil {
		t.Fatal("a container that keeps the chain in its routers was accepted")
	}
	if !strings.Contains(err.Error(), "http.Global") {
		t.Errorf("the error does not name the seam: %v", err)
	}
}

func TestHooksAreCalledWhenThePackageDeclaresThem(t *testing.T) {
	module(t, grpcContainer, map[string]string{
		"tests/internal/harness/harness.go": "package harness\n\n" +
			"func env(v *Variables) {}\n" +
			"func setup() error { return nil }\n" +
			"func resources() []resource.Resource { return nil }\n" +
			"type extra struct{ Drivers any }\n" +
			"func (a *App) boot(ctx context.Context, cut *diogen.Replacements) error { return nil }\n",
	})

	run(t)

	contains(t, "harness_gen.go", generatedFile(t),
		"env(&v)",
		"if err := setup(); err != nil {",
		"append([]resource.Resource{postgres}, resources()...)...",
		"\textra\n",
		"if err := app.boot(ctx, &cut); err != nil {",
	)
}

// Serve exists only for a container that has a transport to serve.
func TestTheBootTestServesOnlyWhatTheContainerHas(t *testing.T) {
	module(t, cacheContainer, nil)

	run(t)

	body := written(t, "tests/boot_test.go")
	contains(t, "boot_test.go", body, "harness.Boot(ctx, diogen.Replacements{})")
	lacks(t, "boot_test.go", body, "Serve()")
	lacks(t, "harness_gen.go", generatedFile(t), "func (a *App) Serve()")
}

// The context is Boot's own, so a container that takes nothing else needs no hook.
func TestTheContextParameterIsBootsOwn(t *testing.T) {
	module(t, contextContainer, nil)

	run(t)

	body := generatedFile(t)
	contains(t, "harness_gen.go", body, "diogen.InitAppWith(ctx, cut)")
	lacks(t, "harness_gen.go", body, "app.params(")
}

// In the container's order, and a name Boot uses itself or a package it imports
// is suffixed. The types are the hook's to spell, so their packages are not
// imported here: they would be unused.
func TestParametersComeFromTheParamsHook(t *testing.T) {
	module(t, paramsContainer, map[string]string{"tests/internal/harness/harness.go": paramsHook})

	run(t)

	body := generatedFile(t)
	contains(t, "harness_gen.go", body,
		"appParam, clientOpts, limits, onClose, err := app.params(ctx)\n\tif err != nil {\n\t\treturn nil, err\n\t}",
		"diogen.InitAppWith(ctx, appParam, clientOpts, limits, onClose, cut)",
	)
	lacks(t, "harness_gen.go", body, `"example.test/shop/client"`, `"example.test/shop/config/di"`)
}

func TestAMissingParamsHookIsRefusedWithItsSignature(t *testing.T) {
	module(t, paramsContainer, nil)

	err := New().Run(t.Context())

	if err == nil {
		t.Fatal("a container with parameters was accepted without the hook that supplies them")
	}
	for _, want := range []string{
		"func (a *App) params(ctx context.Context) (app *di.App, clientOpts *client.Options, limits diogen.Limits, onClose func(name string) error, err error)",
		`client "example.test/shop/client"`,
		`diogen "example.test/shop/config/diogen"`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not hold %q: %v", want, err)
		}
	}
	// The hook goes into harness.go, and a harness.go that exists stops the scaffold.
	contains(t, "boot_test.go", written(t, "tests/boot_test.go"), "harness.Boot(")
	if _, err := os.Stat(filepath.FromSlash("tests/internal/harness/harness_gen.go")); err == nil {
		t.Error("a harness_gen.go that cannot compile was written")
	}
}

func TestNoHookIsCalledWhenNoneIsDeclared(t *testing.T) {
	module(t, grpcContainer, nil)

	run(t)

	lacks(t, "harness_gen.go", generatedFile(t), "env(&v)", "setup()", "resources()", "extra", "app.boot(")
}

// A scaffold deleted on purpose is not written back on every go generate, and
// an edited one is not overwritten; the generated file always is.
func TestALaterRunRewritesOnlyTheGeneratedFile(t *testing.T) {
	module(t, grpcContainer, nil)
	run(t)
	place(t, "tests/boot_test.go", "package tests // mine\n")
	if err := os.Remove(filepath.FromSlash("tests/suite_test.go")); err != nil {
		t.Fatal(err)
	}
	place(t, "config/diogen/di_gen.go", cacheContainer)

	run(t)

	contains(t, "boot_test.go", written(t, "tests/boot_test.go"), "// mine")
	if _, err := os.Stat(filepath.FromSlash("tests/suite_test.go")); err == nil {
		t.Error("a deleted scaffold was written back")
	}
	contains(t, "harness_gen.go", generatedFile(t), "resource.New(postgres, cache)")
}

// go generate runs the directive from the harness package, not from the root.
func TestARunFromTheHarnessPackageWritesAtTheRoot(t *testing.T) {
	module(t, grpcContainer, map[string]string{
		"tests/internal/harness/harness.go": "package harness\n",
	})
	t.Chdir(filepath.FromSlash("tests/internal/harness"))
	beside, err := filepath.Abs(generated)
	if err != nil {
		t.Fatal(err)
	}

	run(t)

	if _, err := os.Stat(beside); err != nil {
		t.Fatalf("the generated file is not beside harness.go: %v", err)
	}
}

func TestAHandWrittenHarnessIsRefusedByName(t *testing.T) {
	module(t, grpcContainer, map[string]string{
		"tests/internal/harness/resources.go": "package harness\n\nfunc Up() {}\n\nvar postgres = 1\n",
	})

	err := New().Run(t.Context())

	if err == nil {
		t.Fatal("a package declaring Up beside the generated one was accepted")
	}
	for _, want := range []string{"resources.go", "Up", "postgres", "hooks"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %q: %v", want, err)
		}
	}
	if _, err := os.Stat(filepath.FromSlash("tests/internal/harness/harness_gen.go")); err == nil {
		t.Error("the generated file was written beside a package it cannot compile with")
	}
}

func TestAnOperandIsRefused(t *testing.T) {
	module(t, grpcContainer, nil)

	err := New().Run(t.Context(), "init")

	if err == nil || !strings.Contains(err.Error(), "init") {
		t.Fatalf("run: %v", err)
	}
}
