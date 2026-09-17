# Variables for the modules enabled in config/di/modules.go.
# Uncommenting a module there is not enough — `waffle` appends the variables it
# needs when a command enables it; by hand, see docs/reference/running-locally.md.

SERVICE={{ .Service }}
ENVIRONMENT=local

# logging — config/service/pry
LOGLEVEL=debug
SENTRY_DSN=
SENTRY_LEVEL=error

# tracing and metrics — config/service/hilton/otel
# empty endpoint means spans and metrics go nowhere
OTEL_EXPORTER_OTLP_ENDPOINT=
HOSTNAME=
