package http

import (
	"net"
	"strconv"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/http/server"
	"gitlab.com/gorib/http/server/middleware"
	"gitlab.com/gorib/waffle/app"
	"gitlab.com/gorib/waffle/tools/http"
)

func InitHttp() {
	di.Wire[server.Mux](server.New)

	// Fallbacks: a service can replace any of these by registering its own
	// di.Wire[middleware.X] (no di.Fallback) — the default is then dropped.
	di.Wire[middleware.Compress](middleware.NewCompress, di.Fallback())
	di.Wire[middleware.Health](middleware.NewHealth, di.Args(map[string]any{
		// Empty → NewHealth uses DefaultHealthPath.
		"path": env.Value("HEALTH_PATH", ""),
	}), di.Fallback())
	di.Wire[middleware.Session](middleware.NewSession, di.Fallback())
	di.Wire[middleware.Tracing](middleware.NewTracing, di.Fallback())
	di.Wire[middleware.Log](middleware.NewLog, di.Fallback())
	di.Wire[middleware.Error](middleware.NewErrorRenderer, di.Fallback())
	di.Wire[middleware.Recover](middleware.NewRecover, di.Fallback())

	di.Wire[*middleware.Standard](middleware.NewStandard)

	di.Wire[net.Addr](net.ResolveTCPAddr, di.For[http.HttpCommand](), di.Args(map[string]any{
		"network": "tcp",
		"address": net.JoinHostPort(
			env.NeedValue[string]("DAEMON_HOST"),
			strconv.FormatUint(uint64(env.NeedValue[uint16]("DAEMON_PORT")), 10),
		),
	}))

	// JSON alone unless the service says otherwise, which it does by registering
	// its own without di.Fallback().
	di.Wire[http.OpenapiOptions](func() http.OpenapiOptions { return nil }, di.Fallback())

	di.Wire[app.BaseCommand](func() app.BaseCommand {
		return app.NewCommand("http", "Start web server")
	}, di.For[http.HttpCommand]())
	di.Wire[http.HttpCommand](http.NewHttpCommand, di.Tag(app.CommandTag))
}
