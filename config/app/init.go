package app

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
)

type (
	commandTag struct{}
)

var (
	CommandTag = commandTag{}
)

func InitApp() {
	di.Wire[app.App](app.Init, di.Args(map[string]any{
		"commands": di.Tags[app.Command](CommandTag),
	}))
}
