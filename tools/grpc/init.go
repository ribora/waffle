package grpc

import (
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
	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("grpc", "Start grpc server") }, di.For[GrpcCommand]())
	di.Wire[GrpcCommand](NewGrpcCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"commands":          di.Tags[ServiceHandler](ServiceTag),
		"middlewares":       di.Tags[Middleware](MiddlewareTag),
		"streamMiddlewares": di.Tags[StreamMiddleware](StreamMiddlewareTag),
	}))
}
