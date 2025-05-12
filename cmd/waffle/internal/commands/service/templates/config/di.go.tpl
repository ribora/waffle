package config

import (
	"fmt"
	"io/fs"
	"net"
	"time"

	_ "github.com/lib/pq"

	"gitlab.trgdev.com/gotrg/white-label/modules/health"
	healthProto "gitlab.trgdev.com/gotrg/white-label/modules/health/proto"

	"gitlab.com/gorib/server/host"
	grpcCommand "gitlab.com/gorib/waffle/tools/grpc"
	migrateCommand "{{ .Project }}/tools/migrate"

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

	di.MustWire[net.Addr](host.NewListener, di.For[grpcCommand.GrpcCommand](), di.Defaults(map[int]any{
		0: "tcp",
		1: env.Value("DAEMON_HOST", "0.0.0.0"),
		2: env.Value("DAEMON_PORT", "8009"),
	}))
	di.MustWire[healthProto.HealthServiceServer](health.NewHealthServiceServer, di.Tag(grpcCommand.ServiceTag))
	grpcCommand.InitGrpc()
	di.MustWire[fs.FS](migrateCommand.NewMigrations, di.For[migrate.MigrateCommand]())
	migrate.InitMigrate()
}
