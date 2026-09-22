module gitlab.com/gorib/waffle

go 1.27.0

require (
	github.com/fatih/color v1.19.0
	github.com/go-co-op/gocron/v2 v2.22.0
	github.com/pborman/getopt/v2 v2.2.0
	github.com/pressly/goose/v3 v3.27.3
	gitlab.com/gorib/amqp v1.9.1
	gitlab.com/gorib/criteria v1.1.0
	gitlab.com/gorib/di v1.9.1
	gitlab.com/gorib/elastic v1.4.0
	gitlab.com/gorib/env v1.0.1
	gitlab.com/gorib/failure v1.3.1
	gitlab.com/gorib/grpc v1.6.1
	gitlab.com/gorib/hilton v1.2.3
	gitlab.com/gorib/http v1.12.2
	gitlab.com/gorib/pry v1.4.1
	gitlab.com/gorib/redis v1.3.0
	gitlab.com/gorib/session v1.2.0
	gitlab.com/gorib/sql v1.8.1
	gitlab.com/gorib/whisk v1.3.2
	golang.org/x/mod v0.40.0
	golang.org/x/tools v0.49.0
	google.golang.org/grpc v1.83.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dave/jennifer v1.7.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/elastic/elastic-transport-go/v8 v8.11.0 // indirect
	github.com/elastic/go-elasticsearch/v8 v8.19.7 // indirect
	github.com/getsentry/sentry-go v0.48.0 // indirect
	github.com/go-chi/chi/v5 v5.3.2 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-sql-driver/mysql v1.10.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jmattheis/goverter v1.9.4 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/rabbitmq/amqp091-go v1.14.0 // indirect
	github.com/redis/go-redis/v9 v9.22.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	github.com/sethvargo/go-retry v0.4.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.45.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.45.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.45.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.45.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.45.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.45.0 // indirect
	go.opentelemetry.io/otel/metric v1.45.0 // indirect
	go.opentelemetry.io/otel/sdk v1.45.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.45.0 // indirect
	go.opentelemetry.io/otel/trace v1.45.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260819154853-08b0e4226688 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260819154853-08b0e4226688 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	modernc.org/libc v1.75.4 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.57.0 // indirect
)

tool (
	github.com/jmattheis/goverter/cmd/goverter
	gitlab.com/gorib/di/cmd/diogen
	gitlab.com/gorib/sql/cmd/store
	gitlab.com/gorib/whisk/cmd/whisk
)
