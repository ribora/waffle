package http

import (
	"net"
	"strconv"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/http/server"
	"gitlab.com/gorib/http/server/middleware"
	"gitlab.com/gorib/waffle/app"
	appConfig "gitlab.com/gorib/waffle/config/app"
	"gitlab.com/gorib/waffle/tools/http"
)

type (
	middlewareTag struct{}
)

var (
	MiddlewareTag = middlewareTag{}
)

func InitHttp() {
	di.Define(middleware.NewCompressMW, di.Tag(MiddlewareTag))
	di.Define(middleware.NewHealthMW, di.Tag(MiddlewareTag))
	di.Define(middleware.NewSessionMW, di.Tag(MiddlewareTag))
	di.Define(middleware.NewTracingMW, di.Tag(MiddlewareTag))
	di.Define(middleware.NewLogMW, di.Tag(MiddlewareTag))
	di.Define(middleware.NewRecoverMW, di.Tag(MiddlewareTag))

	di.Wire[net.Addr](net.ResolveTCPAddr, di.For[http.HttpCommand](), di.Args(map[string]any{
		"network": "tcp",
		"address": net.JoinHostPort(
			env.NeedValue[string]("DAEMON_HOST"),
			strconv.FormatUint(uint64(env.NeedValue[uint16]("DAEMON_PORT")), 10),
		),
	}))

	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("http", "Start web server") }, di.For[http.HttpCommand]())
	di.Wire[http.HttpCommand](http.NewHttpCommand, di.Tag(appConfig.CommandTag), di.Args(map[string]any{
		"middlewares": di.Tags[server.Middleware](MiddlewareTag),
	}))
}
