package app

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
)

func InitApp() {
	di.Wire[app.App](app.Init, di.Args(map[string]any{
		"commands": di.Tags[app.Command](app.CommandTag),
	}))
}
