package scribus

import (
	"time"

	"gitlab.com/gorib/amqp"
	"gitlab.com/gorib/amqp/interceptor"
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/waffle/app"
	"gitlab.com/gorib/waffle/tools/scribus"
)

// Registers only what's common to both modes — does NOT register a
// ScribusCommand. Include tools/scribus/single or tools/scribus/batch.
func InitScribus() {
	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("scribus", "Start RabbitMq consumer") }, di.For[scribus.ScribusCommand]())
	di.Wire[scribus.Consumer](amqp.NewConsumer, di.Args(map[string]any{
		"dsn":        env.NeedValue[string]("RABBIT_DSN"),
		"exchange":   env.NeedValue[string]("RABBIT_EXCHANGE"),
		"queue":      env.NeedValue[string]("RABBIT_QUEUE"),
		"routingKey": "#",
		"opts": []amqp.ConsumerOptions{
			amqp.WithConsumerPageSize(1),
			amqp.WithConsumerFlushInterval(5 * time.Second),
		},
	}))

	// Cross-cutting amqp middlewares (tracing, log, recover) — picked up by
	// scribus via MiddlewareTag and applied around the dispatch handler.
	di.Define(interceptor.NewTracing, di.Tag(scribus.MiddlewareTag))
	di.Define(interceptor.NewLog, di.Tag(scribus.MiddlewareTag))
	di.Define(interceptor.NewRecover, di.Tag(scribus.MiddlewareTag))
}
