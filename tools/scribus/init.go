package scribus

import (
	"time"

	"gitlab.com/gorib/amqp"
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/waffle/app"
)

type commandTag struct{}

var (
	CommandTag = commandTag{}
)

func InitScribus() {
	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("scribus", "Start RabbitMq consumer") }, di.For[ScribusCommand]())
	di.Wire[Consumer](amqp.NewConsumer, di.Args(map[string]any{
		"dsn":        env.NeedValue[string]("RABBIT_DSN"),
		"exchange":   env.NeedValue[string]("RABBIT_EXCHANGE"),
		"queue":      env.NeedValue[string]("RABBIT_QUEUE"),
		"routingKey": "#",
		"opts": []amqp.ConsumerOptions{
			amqp.WithConsumerPageSize(1),
			amqp.WithConsumerFlushInterval(5 * time.Second),
		},
	}))

	// Single handler
	di.Wire[ScribusCommand](NewScribusCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"commands": di.Tags[Handler](CommandTag),
	}))

	// Batch handler
	di.Wire[ScribusCommand](NewBatchScribusCommand, di.Tag(app.CommandTag))
}
