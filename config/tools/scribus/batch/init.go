package batch

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
	"gitlab.com/gorib/waffle/tools/scribus"

	_ "gitlab.com/gorib/waffle/config/tools/scribus"
)

// Register your scribus.BatchHandler implementation alongside this.
func InitBatch() {
	di.Wire[scribus.ScribusCommand](scribus.NewBatchScribusCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"middlewares": di.Tags[scribus.Middleware](scribus.MiddlewareTag),
	}))
}
