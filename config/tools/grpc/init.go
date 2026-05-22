package grpc

import (
	"net"
	"strconv"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/grpc/server"
	"gitlab.com/gorib/grpc/server/interceptor"
	"gitlab.com/gorib/waffle/app"
	appConfig "gitlab.com/gorib/waffle/config/app"
	"gitlab.com/gorib/waffle/tools/grpc"
)

type (
	serviceTag          struct{}
	middlewareTag       struct{}
	streamMiddlewareTag struct{}
	errorMapperTag      struct{}
)

var (
	ServiceTag          = serviceTag{}
	MiddlewareTag       = middlewareTag{}
	StreamMiddlewareTag = streamMiddlewareTag{}
	ErrorMapperTag      = errorMapperTag{}
)

func InitGrpc() {
	di.Wire[server.ErrorMapper](grpc.NewErrorMap, di.Args(map[string]any{
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

	di.Wire[net.Addr](net.ResolveTCPAddr, di.For[grpc.GrpcCommand](), di.Args(map[string]any{
		"network": "tcp",
		"address": net.JoinHostPort(
			env.NeedValue[string]("DAEMON_HOST"),
			strconv.FormatUint(uint64(env.NeedValue[uint16]("DAEMON_PORT")), 10),
		),
	}))

	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("grpc", "Start grpc server") }, di.For[grpc.GrpcCommand]())
	di.Wire[grpc.GrpcCommand](grpc.NewGrpcCommand, di.Tag(appConfig.CommandTag), di.Args(map[string]any{
		"commands":           di.Tags[grpc.ServiceHandler](ServiceTag),
		"interceptors":       di.Tags[server.Interceptor](MiddlewareTag),
		"streamInterceptors": di.Tags[server.StreamInterceptor](StreamMiddlewareTag),
	}))
}
