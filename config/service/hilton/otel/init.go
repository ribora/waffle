package otel

import (
	"github.com/ribora/diogen/di"
	"github.com/ribora/env"
	"github.com/ribora/hilton"
	"github.com/ribora/hilton/otel"
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
