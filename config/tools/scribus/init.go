package scribus

import (
	"github.com/ribora/diogen/di"
	"github.com/ribora/waffle/app"
	"github.com/ribora/waffle/tools/scribus"
)

func InitScribus() {
	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("scribus", "Start the event consumer") }, di.For[scribus.ScribusCommand]())
}
