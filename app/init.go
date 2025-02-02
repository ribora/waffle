package app

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/pry"
)

type commandTag struct{}

var (
	CommandTag = commandTag{}
)

func New() (App, error) {
	err := di.Wire[Configure](func() {}, di.Fallback())
	if err != nil {
		return nil, err
	}
	err = di.Wire[App](func(commands []Command, cfg Configure, logger pry.Logger) (*app, error) {
		return Init(commands, logger)
	}, di.Fallback(), di.Defaults(map[int]any{
		0: di.Tags[Command](CommandTag),
	}))
	if err != nil {
		return nil, err
	}
	application, closer, err := di.NewWithCloser[App]()
	if err != nil {
		return nil, err
	}
	if appInstance, ok := application.(HasCloser); ok {
		appInstance.SetCloser(closer)
	}
	return application, nil
}
