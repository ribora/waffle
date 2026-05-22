package hilton

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/hilton"
)

func InitHilton() {
	di.Wire[hilton.Provider](func() (hilton.Provider, error) {
		return hilton.New(
			hilton.WithService(env.Value("SERVICE", "")),
			hilton.WithEnvironment(env.Value("ENVIRONMENT", "")),
			hilton.WithOtlpGrpc(env.Value("OTEL_EXPORTER_OTLP_ENDPOINT", "")),
		)
	})
}
