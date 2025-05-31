package migrate

import (
	"io/fs"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
)

type migrationsTag struct{}

var MigrationsTag migrationsTag

func InitMigrate() {
	di.MustWire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("migrate", "Run migrations") }, di.For[MigrateCommand]())
	di.MustWire[MigrateCommand](NewMigrateCommand, di.Fallback(), di.Tag(app.CommandTag), di.Defaults(map[int]any{
		0: di.Tags[fs.FS](MigrationsTag),
	}))
}
