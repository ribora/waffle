package http

import (
	"errors"
	"net"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
)

type middlewareTag struct{}

var (
	MiddlewareTag = middlewareTag{}
)

func InitHttp() {
	di.MustWire[MiddlewareList](NewMiddlewareList, di.Defaults(map[int]any{
		0: di.Tags[Middleware](MiddlewareTag),
	}))

	di.MustWire[net.Addr](func() (net.Addr, error) { return nil, errors.New("http listener is not set") }, di.Fallback())

	di.MustWire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("http", "Start web server") }, di.For[HttpCommand]())
	di.MustWire[HttpCommand](NewHttpCommand, di.Fallback(), di.Tag(app.CommandTag))
}
