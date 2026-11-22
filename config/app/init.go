package app

import (
	"io"

	"github.com/ribora/diogen/di"
	"github.com/ribora/waffle/app"
)

func InitApp() {
	di.DefaultCloser[io.Closer](di.Fallback())
	di.Entry[app.App]()

	di.Wire[app.App](app.Init, di.Args(map[string]any{
		"commands": di.Tags(app.CommandTag),
	}))
}
