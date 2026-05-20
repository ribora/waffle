package grpc

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/grpc/server"
	"gitlab.com/gorib/grpc/server/interceptor"
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
	di.Wire[*server.ErrorMap](server.NewErrorMap)

	di.Wire[Middleware](interceptor.NewTracingMW, di.Tag(MiddlewareTag))
	di.Wire[Middleware](interceptor.NewStatusMW, di.Tag(MiddlewareTag))
	di.Wire[Middleware](interceptor.NewSessionMW, di.Tag(MiddlewareTag))
	di.Wire[Middleware](interceptor.NewLogMW, di.Tag(MiddlewareTag))
	di.Wire[Middleware](interceptor.NewRecoverMW, di.Tag(MiddlewareTag))

	di.Wire[StreamMiddleware](interceptor.NewStreamTracingMW, di.Tag(StreamMiddlewareTag))
	di.Wire[StreamMiddleware](interceptor.NewStreamStatusMW, di.Tag(StreamMiddlewareTag))
	di.Wire[StreamMiddleware](interceptor.NewStreamSessionMW, di.Tag(StreamMiddlewareTag))
	di.Wire[StreamMiddleware](interceptor.NewStreamLogMW, di.Tag(StreamMiddlewareTag))
	di.Wire[StreamMiddleware](interceptor.NewStreamRecoverMW, di.Tag(StreamMiddlewareTag))

	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("grpc", "Start grpc server") }, di.For[GrpcCommand]())
	di.Wire[GrpcCommand](NewGrpcCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"commands":           di.Tags[ServiceHandler](ServiceTag),
		"interceptors":       di.Tags[Middleware](MiddlewareTag),
		"streamInterceptors": di.Tags[StreamMiddleware](StreamMiddlewareTag),
	}))
}
