package amqp

import (
	"github.com/ribora/amqp"
	"github.com/ribora/diogen/di"
	"github.com/ribora/env"
)

func InitAmqp() {
	di.Define(amqp.NewPublisher, di.Alias[amqp.Publisher](), di.Args(map[string]any{
		"dsn":      env.NeedValue[string]("RABBIT_DSN"),
		"exchange": env.NeedValue[string]("RABBIT_EXCHANGE"),
	}), di.Fallback())
}
