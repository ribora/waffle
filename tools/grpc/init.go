package grpc

import (
	"net"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/server/host"
	"gitlab.com/gorib/waffle/app"
)

type serviceTag struct{}
type middlewareTag struct{}

var (
	ServiceTag    = serviceTag{}
	MiddlewareTag = middlewareTag{}
)

func InitGrpc() {
	di.Wire[net.Addr](host.NewListener, di.For[GrpcCommand](), di.Fallback(), di.Defaults(map[int]any{
		0: "",
		1: "",
		2: "",
	}))

	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("grpc", "Start grpc server") }, di.For[GrpcCommand]())
	di.Wire[GrpcCommand](NewGrpcCommand, di.Tag(app.CommandTag), di.Fallback(), di.Defaults(map[int]any{
		0: di.Tags[ServiceHandler](ServiceTag),
		1: di.Tags[Middleware](MiddlewareTag),
	}))
}
