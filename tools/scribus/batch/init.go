package batch

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
	"gitlab.com/gorib/waffle/tools/scribus"
)

// Register your scribus.BatchHandler implementation alongside this.
func InitBatch() {
	di.Include("gitlab.com/gorib/waffle/tools/scribus")
	di.Wire[scribus.ScribusCommand](scribus.NewBatchScribusCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"middlewares": di.Tags[scribus.Middleware](scribus.MiddlewareTag),
	}))
}
