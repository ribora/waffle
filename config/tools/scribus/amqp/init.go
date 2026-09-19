package amqp

import (
	"time"

	"gitlab.com/gorib/amqp"
	"gitlab.com/gorib/amqp/interceptor"
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/waffle/tools/scribus"
)

func InitScribusAmqp() {
	di.Wire[scribus.Consumer](amqp.NewConsumer, di.Args(map[string]any{
		"dsn":      env.NeedValue[string]("RABBIT_DSN"),
		"exchange": env.NeedValue[string]("RABBIT_EXCHANGE"),
		"queue":    env.NeedValue[string]("RABBIT_QUEUE"),
		"opts": []amqp.ConsumerOptions{
			amqp.WithConsumerPageSize(1),
			amqp.WithConsumerFlushInterval(5 * time.Second),
		},
	}))
	di.Wire[scribus.Subjects](amqp.Subjects)

	di.Define(interceptor.NewTracing, di.Tag(scribus.MiddlewareTag))
	di.Define(interceptor.NewLog, di.Tag(scribus.MiddlewareTag))
	di.Define(interceptor.NewRecover, di.Tag(scribus.MiddlewareTag))
}
