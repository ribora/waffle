package scribus

import (
	"time"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/server/rabbit"
	"gitlab.com/gorib/waffle/app"
)

type commandTag struct{}

var (
	CommandTag = commandTag{}
)

func InitScribus() {
	di.Wire[BusConsumer](rabbit.NewBusConsumer, di.Fallback(), di.Defaults(map[int]any{
		0: 30 * time.Second,
	}))
	di.Wire[rabbit.Topic](rabbit.NewTopic, di.Fallback(), di.Defaults(map[int]any{
		0: "",
		1: "",
		2: "",
		3: "",
	}))

	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("scribus", "Start RabbitMq consumer") }, di.For[ScribusCommand]())
	di.Wire[ScribusCommand](NewScribusCommand, di.Tag(app.CommandTag), di.Fallback(), di.Defaults(map[int]any{
		0: di.Tags[rabbit.Handler](CommandTag),
	}))
}
