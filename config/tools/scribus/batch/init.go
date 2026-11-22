package batch

import (
	"github.com/ribora/diogen/di"
	"github.com/ribora/waffle/app"
	"github.com/ribora/waffle/tools/scribus"

	_ "github.com/ribora/waffle/config/tools/scribus"
)

func InitBatch() {
	di.Wire[scribus.ScribusCommand](scribus.NewBatchScribusCommand, di.Tag(app.CommandTag))
}
