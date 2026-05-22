package hilton

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/hilton"
)

func InitHilton() {
	di.Wire[hilton.Provider](func() (hilton.Provider, error) {
		service := env.Value("SERVICE", "")
		environment := env.Value("ENVIRONMENT", "")

		credentials := hilton.NewInsecureCredentials()
		endpoint := env.Value("OTEL_EXPORTER_OTLP_ENDPOINT", "")

		metricExporter, err := hilton.NewGrpcMetricExporter(endpoint, credentials)
		if err != nil {
			return nil, err
		}

		traceExporter, err := hilton.NewGrpcTraceExporter(endpoint, credentials)
		if err != nil {
			return nil, err
		}

		return hilton.New(service, environment, traceExporter, metricExporter)
	})
}
