package migrate

import (
	"github.com/ribora/diogen/di"
	"github.com/ribora/waffle/app"
	"github.com/ribora/waffle/tools/migrate"

	// The command runs against a sql.Db, which names the goose dialect.
	_ "github.com/ribora/waffle/config/service/sql"
)

func InitMigrate() {
	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("migrate", "Run migrations") }, di.For[migrate.MigrateCommand]())
	di.Wire[migrate.MigrateCommand](migrate.NewMigrateCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"migrations": di.Tags(migrate.MigrationsTag),
	}))
}
