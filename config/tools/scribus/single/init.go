package single

import (
	"github.com/ribora/diogen/di"
	"github.com/ribora/waffle/app"
	"github.com/ribora/waffle/tools/scribus"

	_ "github.com/ribora/waffle/config/tools/scribus"
)

func InitSingle() {
	di.Wire[scribus.ScribusCommand](scribus.NewScribusCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"commands": di.Tags(scribus.CommandTag),
	}))
}
