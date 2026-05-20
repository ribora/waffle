package scribus

import (
	"time"

	"gitlab.com/gorib/amqp"
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/waffle/app"
)

type commandTag struct{}
type middlewareTag struct{}

var (
	CommandTag    = commandTag{}
	MiddlewareTag = middlewareTag{}
)

// Registers only what's common to both modes — does NOT register a
// ScribusCommand. Include tools/scribus/single or tools/scribus/batch.
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
}
