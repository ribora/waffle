package single

import (
	"gitlab.com/gorib/di"
	appConfig "gitlab.com/gorib/waffle/config/app"
	scribusConfig "gitlab.com/gorib/waffle/config/tools/scribus"
	"gitlab.com/gorib/waffle/tools/scribus"
)

// Routes each key to a dedicated scribus.Handler tagged with scribus.CommandTag.
func InitSingle() {
	di.Include("gitlab.com/gorib/waffle/tools/scribus")
	di.Wire[scribus.ScribusCommand](scribus.NewScribusCommand, di.Tag(appConfig.CommandTag), di.Args(map[string]any{
		"commands":    di.Tags[scribus.Handler](scribusConfig.CommandTag),
		"middlewares": di.Tags[scribus.Middleware](scribusConfig.MiddlewareTag),
	}))
}
