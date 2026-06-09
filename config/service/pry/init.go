package pry

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/pry/channels"
)

func InitPry() {
	di.Wire[pry.Logger](func() (pry.Logger, error) {
		dsn := env.Value("SENTRY_DSN", "")
		environment := env.Value("ENVIRONMENT", "noenv")
		service := env.Value("SERVICE", "nocontour")
		sentry, err := channels.Sentry("error", channels.WithSentryConnection(dsn, environment, channels.WithSentryService(service)))
		if err != nil {
			return nil, err
		}
		return pry.New(env.Value("LOGLEVEL", "info"), pry.ToChannels(sentry))
	}, di.Fallback())
}
