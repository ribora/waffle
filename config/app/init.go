package app

import (
	"io"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
)

func InitApp() {
	di.DefaultCloser[io.Closer](di.Fallback())
	di.Entry[app.App]()

	di.Wire[app.App](app.Init, di.Args(map[string]any{
		"commands": di.Tags(app.CommandTag),
	}))
}
