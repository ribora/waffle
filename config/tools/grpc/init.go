package grpc

import (
	"net"
	"strconv"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/grpc/server"
	"gitlab.com/gorib/grpc/server/interceptor"
	"gitlab.com/gorib/waffle/app"
	"gitlab.com/gorib/waffle/tools/grpc"
)

func InitGrpc() {
	di.Wire[server.ErrorMapper](grpc.NewErrorMap, di.Args(map[string]any{
		"mappers": di.Tags[server.ErrorMapper](grpc.ErrorMapperTag),
	}))

	di.Define(interceptor.NewTracingMW, di.Tag(grpc.MiddlewareTag))
	di.Define(interceptor.NewStatusMW, di.Tag(grpc.MiddlewareTag))
	di.Define(interceptor.NewSessionMW, di.Tag(grpc.MiddlewareTag))
	di.Define(interceptor.NewLogMW, di.Tag(grpc.MiddlewareTag))
	di.Define(interceptor.NewRecoverMW, di.Tag(grpc.MiddlewareTag))

	di.Define(interceptor.NewStreamTracingMW, di.Tag(grpc.StreamMiddlewareTag))
	di.Define(interceptor.NewStreamStatusMW, di.Tag(grpc.StreamMiddlewareTag))
	di.Define(interceptor.NewStreamSessionMW, di.Tag(grpc.StreamMiddlewareTag))
	di.Define(interceptor.NewStreamLogMW, di.Tag(grpc.StreamMiddlewareTag))
	di.Define(interceptor.NewStreamRecoverMW, di.Tag(grpc.StreamMiddlewareTag))

	di.Wire[net.Addr](net.ResolveTCPAddr, di.For[grpc.GrpcCommand](), di.Args(map[string]any{
		"network": "tcp",
		"address": net.JoinHostPort(
			env.NeedValue[string]("DAEMON_HOST"),
			strconv.FormatUint(uint64(env.NeedValue[uint16]("DAEMON_PORT")), 10),
		),
	}))

	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("grpc", "Start grpc server") }, di.For[grpc.GrpcCommand]())
	di.Wire[grpc.GrpcCommand](grpc.NewGrpcCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"commands":           di.Tags[grpc.ServiceHandler](grpc.ServiceTag),
		"interceptors":       di.Tags[server.Interceptor](grpc.MiddlewareTag),
		"streamInterceptors": di.Tags[server.StreamInterceptor](grpc.StreamMiddlewareTag),
	}))
}
