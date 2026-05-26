package migrate

import (
	"io/fs"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
	"gitlab.com/gorib/waffle/tools/migrate"
)

func InitMigrate() {
	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("migrate", "Run migrations") }, di.For[migrate.MigrateCommand]())
	di.Wire[migrate.MigrateCommand](migrate.NewMigrateCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"migrations": di.Tags[fs.FS](migrate.MigrationsTag),
	}))
}
