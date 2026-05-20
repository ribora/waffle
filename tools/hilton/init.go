// Package hilton wires a hilton.Provider into the DI graph.
// Reading OTEL_EXPORTER_OTLP_ENDPOINT, SERVICE and ENVIRONMENT from env.
// Without an endpoint the provider is real but exports nowhere — spans and
// metrics are created and propagated via traceparent but discarded.
//
// Application:
//
//	di.Include("gitlab.com/gorib/waffle/tools/hilton")
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
