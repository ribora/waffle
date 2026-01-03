package config

import (
	"fmt"
	"io/fs"
	"net"
	"strconv"
	"time"

	grpcCommand "gitlab.com/gorib/waffle/tools/grpc"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/pry/channels"
	"gitlab.com/gorib/storage/sql"
	"gitlab.com/gorib/waffle/tools/migrate"
)

func InitDi() {
	di.MustWire[pry.Logger](func() (pry.Logger, error) {
		environment := fmt.Sprintf("%s_%s", env.NeedValue[string]("GITLAB_ENVIRONMENT_NAME"), env.NeedValue[string]("ENVIRONMENT"))
		sentry, err := channels.Sentry("error", channels.WithSentryConnection(env.Value("SENTRY_DSN", ""), environment))
		if err != nil {
			return nil, err
		}
		return pry.New(env.Value("LOGLEVEL", "info"), pry.ToChannels(sentry))
	})

	di.MustWire[sql.Db](sql.NewDb, di.Defaults(map[int]any{
		0: env.NeedValue[string]("DATABASE_DRIVER"),
		1: fmt.Sprintf("%s?dbname=%s&sslmode=disable", env.NeedValue[string]("DATABASE_DSN"), env.NeedValue[string]("DATABASE_NAME")),
		2: env.Value("DATABASE_TIMEOUT", time.Second),
	}))

	di.MustWire[net.Addr](net.ResolveTCPAddr, di.For[grpcCommand.GrpcCommand](), di.Defaults(map[int]any{
		0: "tcp",
		1: net.JoinHostPort(
			env.NeedValue[string]("DAEMON_HOST"),
			strconv.FormatUint(uint64(env.NeedValue[uint16]("DAEMON_PORT")), 10),
		),
	}))

	migrate.InitMigrate()
}
