package app

import (
	"gitlab.com/gorib/di"
)

type commandTag struct{}

var (
	CommandTag = commandTag{}
)

func InitApp() {
	di.Wire[App](Init, di.Args(map[string]any{
		"commands": di.Tags[Command](CommandTag),
	}))
}
