package scribus

import (
	"time"

	"gitlab.com/gorib/amqp"
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
)

type commandTag struct{}

var (
	CommandTag = commandTag{}
)

func InitScribus() {
	initCommand()
	di.MustWire[ScribusCommand](NewScribusCommand, di.Tag(app.CommandTag), di.Fallback(), di.Defaults(map[int]any{
		0: di.Tags[Handler](CommandTag),
	}))
}

func InitBatchScribus() {
	initCommand()
	di.MustWire[ScribusCommand](NewBatchScribusCommand, di.Tag(app.CommandTag), di.Fallback())
}

func initCommand() {
	di.MustWire[Consumer](amqp.NewConsumer, di.Fallback(), di.Defaults(map[int]any{
		0: "",
		1: "",
		2: "",
		3: "",
		4: 1,
		5: 5 * time.Second,
	}))

	di.MustWire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("scribus", "Start RabbitMq consumer") }, di.For[ScribusCommand]())
}
