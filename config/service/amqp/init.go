package amqp

import (
	"gitlab.com/gorib/amqp"
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
)

func InitAmqp() {
	di.Define(amqp.NewPublisher, di.Alias[amqp.Publisher](), di.Args(map[string]any{
		"dsn":      env.NeedValue[string]("RABBIT_DSN"),
		"exchange": env.NeedValue[string]("RABBIT_EXCHANGE"),
	}), di.Fallback())
}
