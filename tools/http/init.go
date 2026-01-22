package http

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
)

type middlewareTag struct{}

var (
	MiddlewareTag = middlewareTag{}
)

func InitHttp() {
	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("http", "Start web server") }, di.For[HttpCommand]())
	di.Wire[MiddlewareList](NewMiddlewareList, di.Args(map[string]any{
		"middlewares": di.Tags[Middleware](MiddlewareTag),
	}))
	di.Wire[HttpCommand](NewHttpCommand, di.Tag(app.CommandTag))
}
