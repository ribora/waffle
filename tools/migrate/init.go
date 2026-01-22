package migrate

import (
	"io/fs"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
)

type migrationsTag struct{}

var MigrationsTag migrationsTag

func InitMigrate() {
	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("migrate", "Run migrations") }, di.For[MigrateCommand]())
	di.Wire[MigrateCommand](NewMigrateCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"migrations": di.Tags[fs.FS](MigrationsTag),
	}))
}
