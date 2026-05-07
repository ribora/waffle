module gitlab.com/gorib/waffle

go 1.25.0

require (
	github.com/fatih/color v1.18.0
	github.com/gofiber/fiber/v2 v2.52.9
	github.com/pborman/getopt/v2 v2.1.0
	github.com/pressly/goose/v3 v3.24.1
	gitlab.com/gorib/amqp v1.0.1
	gitlab.com/gorib/di v1.3.5
	gitlab.com/gorib/env v1.0.0
	gitlab.com/gorib/grpc v1.0.0
	gitlab.com/gorib/http v1.0.0
	gitlab.com/gorib/pry v1.0.1
	gitlab.com/gorib/session v1.0.0
	gitlab.com/gorib/storage v1.0.0
	gitlab.com/gorib/try v0.0.0-20250224141908-512acf4daa35
	golang.org/x/exp v0.0.0-20250620022241-b7579e27df2b
	google.golang.org/grpc v1.77.0
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/dave/jennifer v1.7.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.8.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/klauspost/compress v1.18.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/rabbitmq/amqp091-go v1.10.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	github.com/samber/lo v1.52.0 // indirect
	github.com/sethvargo/go-retry v0.3.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.68.0 // indirect
	gitlab.com/gorib/criteria v0.0.0-20250224141747-7755bfaa022c // indirect
	gitlab.com/gorib/failure v1.0.0 // indirect
	gitlab.com/gorib/gorival v1.2.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251111163417-95abcf5c77ba // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	modernc.org/libc v1.66.10 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.42.2 // indirect
)

tool (
	gitlab.com/gorib/di/cmd/diogen
	gitlab.com/gorib/gorival/cmd/gorival
	gitlab.com/gorib/storage/sql/cmd/store
)
