package http

import (
	"net"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/server/host"
	"gitlab.com/gorib/waffle/app"
)

type middlewareTag struct{}

var (
	MiddlewareTag = middlewareTag{}
)

func InitHttp() {
	di.Wire[MiddlewareList](NewMiddlewareList, di.Defaults(map[int]any{
		0: di.Tags[Middleware](MiddlewareTag),
	}))

	di.Wire[net.Addr](host.NewListener, di.For[HttpCommand](), di.Fallback(), di.Defaults(map[int]any{
		0: "",
		1: "",
		2: "",
	}))

	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("http", "Start web server") }, di.For[HttpCommand]())
	di.Wire[HttpCommand](NewHttpCommand, di.Fallback(), di.Tag(app.CommandTag))
}
