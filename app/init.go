package app

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/try"
)

type commandTag struct{}

var (
	CommandTag = commandTag{}
)

func New() App {
	di.Wire[Configure](func() {}, di.Fallback())
	di.Wire[App](func(commands []Command, cfg Configure, logger pry.Logger) (*app, error) {
		return Init(commands, logger)
	}, di.Fallback(), di.Defaults(map[int]any{
		0: di.Tags[Command](CommandTag),
	}))
	application, closer, err := di.NewWithCloser[App]()
	try.ThrowError(err)
	if appInstance, ok := application.(*app); ok {
		appInstance.closer = closer
	}
	return application
}
