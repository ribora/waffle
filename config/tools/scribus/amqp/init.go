package amqp

import (
	"time"

	"github.com/ribora/amqp"
	"github.com/ribora/amqp/interceptor"
	"github.com/ribora/diogen/di"
	"github.com/ribora/env"
	"github.com/ribora/waffle/tools/scribus"
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

	tracing := di.Define(interceptor.NewTracing)
	logging := di.Define(interceptor.NewLog)
	recovering := di.Define(interceptor.NewRecover)

	di.Wire[scribus.Middlewares](scribus.NewMiddlewares, di.Fallback(), di.Args(map[string]any{
		"middlewares": di.List(tracing, logging, recovering),
	}))
}
