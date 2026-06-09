package otel

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/hilton/otel"
)

func InitOtel() {
	di.Wire[hilton.Provider](func() (hilton.Provider, error) {
		service := env.Value("SERVICE", "")
		environment := env.Value("ENVIRONMENT", "")

		credentials := otel.NewInsecureCredentials()
		endpoint := env.Value("OTEL_EXPORTER_OTLP_ENDPOINT", "")

		metricExporter, err := otel.NewGrpcMetricExporter(endpoint, credentials)
		if err != nil {
			return nil, err
		}

		traceExporter, err := otel.NewGrpcTraceExporter(endpoint, credentials)
		if err != nil {
			return nil, err
		}

		return otel.NewProvider(service, environment, traceExporter, metricExporter,
			otel.WithInstanceID(env.Value("HOSTNAME", "")),
		)
	}, di.Fallback())
}
