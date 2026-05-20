package single

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
	"gitlab.com/gorib/waffle/tools/scribus"
)

// Routes each key to a dedicated scribus.Handler tagged with scribus.CommandTag.
func InitSingle() {
	di.Include("gitlab.com/gorib/waffle/tools/scribus")
	di.Wire[scribus.ScribusCommand](scribus.NewScribusCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"commands":    di.Tags[scribus.Handler](scribus.CommandTag),
		"middlewares": di.Tags[scribus.Middleware](scribus.MiddlewareTag),
	}))
}
