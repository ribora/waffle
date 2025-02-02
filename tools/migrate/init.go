package migrate

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
)

func InitMigrate() {
	di.MustWire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("migrate", "Run migrations") }, di.For[MigrateCommand]())
	di.MustWire[MigrateCommand](NewMigrateCommand, di.Fallback(), di.Tag(app.CommandTag))
}
