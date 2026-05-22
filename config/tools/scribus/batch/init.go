package batch

import (
	"gitlab.com/gorib/di"
	appConfig "gitlab.com/gorib/waffle/config/app"
	scribusConfig "gitlab.com/gorib/waffle/config/tools/scribus"
	"gitlab.com/gorib/waffle/tools/scribus"
)

// Register your scribus.BatchHandler implementation alongside this.
func InitBatch() {
	di.Include("gitlab.com/gorib/waffle/tools/scribus")
	di.Wire[scribus.ScribusCommand](scribus.NewBatchScribusCommand, di.Tag(appConfig.CommandTag), di.Args(map[string]any{
		"middlewares": di.Tags[scribus.Middleware](scribusConfig.MiddlewareTag),
	}))
}
