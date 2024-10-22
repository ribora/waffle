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
	"gitlab.com/gorib/storage"
	"gitlab.com/gorib/storage/sql"
	"gitlab.com/gorib/try"
	"gitlab.com/gorib/waffle/tools/migrate"
)

func InitDi() {
	di.Wire[pry.Logger](func() (pry.Logger, error) {
		environment := fmt.Sprintf("%s_%s", env.NeedValue[string]("GITLAB_ENVIRONMENT_NAME"), env.NeedValue[string]("ENVIRONMENT"))
		return pry.New(
			env.Value("LOGLEVEL", "info"),
			pry.ToChannels(try.Throw(channels.Sentry(env.Value("SENTRY_DSN", ""), environment))),
		)
	})

	di.Wire[sql.Db](sql.NewDb, di.Defaults(map[int]any{
		0: env.NeedValue[string]("DATABASE_DRIVER"),
		1: fmt.Sprintf("%s?dbname=%s&sslmode=disable", env.NeedValue[string]("DATABASE_DSN"), env.NeedValue[string]("DATABASE_NAME")),
		2: env.Value("DATABASE_TIMEOUT", time.Second),
	}))
	di.Wire[storage.Repository](sql.NewRepository)


	di.Wire[net.Addr](host.NewListener, di.For[grpcCommand.GrpcCommand](), di.Defaults(map[int]any{
		0: "tcp",
		1: env.Value("DAEMON_HOST", "0.0.0.0"),
		2: env.Value("DAEMON_PORT", "8009"),
	}))
	di.Wire[healthProto.HealthServiceServer](health.NewHealthServiceServer, di.Tag(grpcCommand.ServiceTag))
	grpcCommand.InitGrpc()

	di.Wire[fs.FS](migrateCommand.NewMigrations, di.For[migrate.MigrateCommand]())
	migrate.InitMigrate()
}
