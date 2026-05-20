//go:generate go tool diogen -output config/diogen/di_gen.go -pkg diogen $GOFILE
package di

import (
	"fmt"
	"net"
	"strconv"

	grpcCommand "gitlab.com/gorib/waffle/tools/grpc"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/pry/channels"
	"gitlab.com/gorib/storage/sql"
)

func InitDi() {
	di.Include("gitlab.com/gorib/waffle/app")
	di.Include("gitlab.com/gorib/waffle/tools/grpc")
	di.Include("gitlab.com/gorib/waffle/tools/migrate")

	di.Wire[pry.Logger](func() (pry.Logger, error) {
		dsn := env.Value("SENTRY_DSN", "")
		environment := env.Value("ENVIRONMENT", "noenv")
		serviceName := env.Value("SERVICE", "nocontour")
		sentry, err := channels.Sentry("error", channels.WithSentryConnection(dsn, environment, channels.WithSentryService(serviceName)))
		if err != nil {
			return nil, err
		}
		return pry.New(env.Value("LOGLEVEL", "info"), pry.ToChannels(sentry))
	})

	di.Wire[sql.Db](sql.NewDb, di.Args(map[string]any{
		"driver": env.NeedValue[string]("DATABASE_DRIVER"),
		"dsn":    fmt.Sprintf("%s?dbname=%s&sslmode=disable", env.NeedValue[string]("DATABASE_DSN"), env.NeedValue[string]("DATABASE_NAME")),
	}))

	di.Wire[net.Addr](net.ResolveTCPAddr, di.For[grpcCommand.GrpcCommand](), di.Args(map[string]any{
		"network": "tcp",
		"address": net.JoinHostPort(
			env.NeedValue[string]("DAEMON_HOST"),
			strconv.FormatUint(uint64(env.NeedValue[uint16]("DAEMON_PORT")), 10),
		),
	}))
}
