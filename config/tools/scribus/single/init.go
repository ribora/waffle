package single

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
	"gitlab.com/gorib/waffle/tools/scribus"

	_ "gitlab.com/gorib/waffle/config/tools/scribus"
)

func InitSingle() {
	di.Wire[scribus.ScribusCommand](scribus.NewScribusCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"commands": di.Tags(scribus.CommandTag),
	}))
}
