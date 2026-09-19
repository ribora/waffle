package scribus

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
	"gitlab.com/gorib/waffle/tools/scribus"
)

func InitScribus() {
	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("scribus", "Start the event consumer") }, di.For[scribus.ScribusCommand]())
}
