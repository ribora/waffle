package migrate

import (
	"io/fs"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
	appConfig "gitlab.com/gorib/waffle/config/app"
	"gitlab.com/gorib/waffle/tools/migrate"
)

type migrationsTag struct{}

var MigrationsTag migrationsTag

func InitMigrate() {
	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("migrate", "Run migrations") }, di.For[migrate.MigrateCommand]())
	di.Wire[migrate.MigrateCommand](migrate.NewMigrateCommand, di.Tag(appConfig.CommandTag), di.Args(map[string]any{
		"migrations": di.Tags[fs.FS](MigrationsTag),
	}))
}
