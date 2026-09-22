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
		"mappers": di.Tags(grpc.ErrorMapperTag),
	}))

	session := di.Define(interceptor.NewSessionMW)
	tracing := di.Define(interceptor.NewTracingMW)
	logging := di.Define(interceptor.NewLogMW)
	status := di.Define(interceptor.NewStatusMW)
	recovering := di.Define(interceptor.NewRecoverMW)

	streamSession := di.Define(interceptor.NewStreamSessionMW)
	streamTracing := di.Define(interceptor.NewStreamTracingMW)
	streamLogging := di.Define(interceptor.NewStreamLogMW)
	streamStatus := di.Define(interceptor.NewStreamStatusMW)
	streamRecovering := di.Define(interceptor.NewStreamRecoverMW)

	di.Wire[server.Interceptors](grpc.NewInterceptors, di.Fallback(), di.Args(map[string]any{
		"interceptors": di.List(session, tracing, logging, status, recovering),
	}))
	di.Wire[server.StreamInterceptors](grpc.NewStreamInterceptors, di.Fallback(), di.Args(map[string]any{
		"interceptors": di.List(streamSession, streamTracing, streamLogging, streamStatus, streamRecovering),
	}))

	di.Wire[net.Addr](net.ResolveTCPAddr, di.For[grpc.GrpcCommand](), di.Args(map[string]any{
		"network": "tcp",
		"address": net.JoinHostPort(
			env.NeedValue[string]("DAEMON_HOST"),
			strconv.FormatUint(uint64(env.NeedValue[uint16]("DAEMON_PORT")), 10),
		),
	}))

	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("grpc", "Start grpc server") }, di.For[grpc.GrpcCommand]())
	di.Wire[grpc.GrpcCommand](grpc.NewGrpcCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"commands": di.Tags(grpc.ServiceTag),
	}))
}
