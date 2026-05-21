package http

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/http/server/middleware"
	"gitlab.com/gorib/waffle/app"
)

type middlewareTag struct{}

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

	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("http", "Start web server") }, di.For[HttpCommand]())
	di.Wire[MiddlewareList](NewMiddlewareList, di.Args(map[string]any{
		"middlewares": di.Tags[Middleware](MiddlewareTag),
	}))
	di.Wire[HttpCommand](NewHttpCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"middlewares": di.Tags[Middleware](MiddlewareTag),
	}))
}
