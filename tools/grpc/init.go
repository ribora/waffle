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
type errorMapperTag struct{}

var (
	ServiceTag          = serviceTag{}
	MiddlewareTag       = middlewareTag{}
	StreamMiddlewareTag = streamMiddlewareTag{}
	ErrorMapperTag      = errorMapperTag{}
)

func InitGrpc() {
	di.Wire[server.ErrorMapper](NewErrorMap, di.Args(map[string]any{
		"mappers": di.Tags[server.ErrorMapper](ErrorMapperTag),
	}))

	di.Define(interceptor.NewTracingMW, di.Tag(MiddlewareTag))
	di.Define(interceptor.NewStatusMW, di.Tag(MiddlewareTag))
	di.Define(interceptor.NewSessionMW, di.Tag(MiddlewareTag))
	di.Define(interceptor.NewLogMW, di.Tag(MiddlewareTag))
	di.Define(interceptor.NewRecoverMW, di.Tag(MiddlewareTag))

	di.Define(interceptor.NewStreamTracingMW, di.Tag(StreamMiddlewareTag))
	di.Define(interceptor.NewStreamStatusMW, di.Tag(StreamMiddlewareTag))
	di.Define(interceptor.NewStreamSessionMW, di.Tag(StreamMiddlewareTag))
	di.Define(interceptor.NewStreamLogMW, di.Tag(StreamMiddlewareTag))
	di.Define(interceptor.NewStreamRecoverMW, di.Tag(StreamMiddlewareTag))

	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("grpc", "Start grpc server") }, di.For[GrpcCommand]())
	di.Wire[GrpcCommand](NewGrpcCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"commands":           di.Tags[ServiceHandler](ServiceTag),
		"interceptors":       di.Tags[server.Interceptor](MiddlewareTag),
		"streamInterceptors": di.Tags[server.StreamInterceptor](StreamMiddlewareTag),
	}))
}
