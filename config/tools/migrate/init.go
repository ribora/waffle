package migrate

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/waffle/app"
	"gitlab.com/gorib/waffle/tools/migrate"

	// The command runs against a sql.Db, and the driver names the goose dialect.
	_ "gitlab.com/gorib/waffle/config/service/sql"
)

func InitMigrate() {
	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("migrate", "Run migrations") }, di.For[migrate.MigrateCommand]())
	di.Wire[migrate.MigrateCommand](migrate.NewMigrateCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"migrations": di.Tags(migrate.MigrationsTag),
		"driver":     env.NeedValue[string]("DB_DRIVER"),
	}))
}
