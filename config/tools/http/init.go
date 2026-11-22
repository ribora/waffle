package http

import (
	"net"
	"strconv"

	"github.com/ribora/diogen/di"
	"github.com/ribora/env"
	"github.com/ribora/http/server"
	"github.com/ribora/http/server/middleware"
	"github.com/ribora/waffle/app"
	"github.com/ribora/waffle/tools/http"
)

func InitHttp() {
	di.Wire[server.Mux](server.New)

	compress := di.Define(middleware.NewCompress)
	health := di.Define(middleware.NewHealth, di.Args(map[string]any{
		// Empty → NewHealth uses DefaultHealthPath.
		"path": env.Value("HEALTH_PATH", ""),
	}))
	session := di.Define(middleware.NewSession)
	tracing := di.Define(middleware.NewTracing)
	logging := di.Define(middleware.NewLog)
	rendering := di.Define(middleware.NewErrorRenderer)
	recovering := di.Define(middleware.NewRecover)

	di.Wire[http.Global](http.NewGlobal, di.Fallback(), di.Args(map[string]any{
		"layers": di.List(compress, health, session, tracing, logging, rendering, recovering),
	}))

	di.Wire[net.Addr](net.ResolveTCPAddr, di.For[http.HttpCommand](), di.Args(map[string]any{
		"network": "tcp",
		"address": net.JoinHostPort(
			env.NeedValue[string]("DAEMON_HOST"),
			strconv.FormatUint(uint64(env.NeedValue[uint16]("DAEMON_PORT")), 10),
		),
	}))

	di.Wire[app.BaseCommand](func() app.BaseCommand {
		return app.NewCommand("http", "Start web server")
	}, di.For[http.HttpCommand]())
	di.Wire[http.HttpCommand](http.NewHttpCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"routers": di.Tags(http.RouterTag),
	}))
}
