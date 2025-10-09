package grpc

import (
	"errors"
	"net"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
)

type serviceTag struct{}
type middlewareTag struct{}
type streamMiddlewareTag struct{}

var (
	ServiceTag          = serviceTag{}
	MiddlewareTag       = middlewareTag{}
	StreamMiddlewareTag = streamMiddlewareTag{}
)

func InitGrpc() {
	di.MustWire[net.Addr](func() (net.Addr, error) { return nil, errors.New("grpc listener is not set") }, di.Fallback())

	di.MustWire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("grpc", "Start grpc server") }, di.For[GrpcCommand]())
	di.MustWire[GrpcCommand](NewGrpcCommand, di.Tag(app.CommandTag), di.Fallback(), di.Defaults(map[int]any{
		0: di.Tags[ServiceHandler](ServiceTag),
		1: di.Tags[Middleware](MiddlewareTag),
		2: di.Tags[StreamMiddleware](StreamMiddlewareTag),
	}))
}
