package app

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/try"
)

type commandTag struct{}

var (
	CommandTag = commandTag{}
)

func New() App {
	di.Wire[Configure](func() {}, di.Fallback())
	di.Wire[App](func(commands []Command, cfg Configure) App { return try.Throw(Init(commands)) }, di.Fallback(), di.Defaults(map[int]any{
		0: di.Tags[Command](CommandTag),
	}))
	application, closer := di.NewWithCloser[App]()
	if appInstance, ok := application.(*app); ok {
		appInstance.closer = closer
	}
	return application
}
