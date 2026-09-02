# What a service needs to run

Adding a blank import to `config/di/modules.go` wires a module in. Most modules
also expect something to be running and something to be in `.env` — and neither
is visible from the import line. This page is that list.

Everything below is read by the module's own `Init…()`. Variables your service
reads itself go in [Environment Variables](environment.md); this page is only
about what the framework asks for.

## By module

| Blank import | Needs running | Required in `.env` | Optional |
|---|---|---|---|
| `config/app` | — | — | — |
| `config/service/pry` | — | — | `LOGLEVEL` (`info`), `ENVIRONMENT` (`noenv`), `SERVICE` (`nocontour`), `SENTRY_DSN` |
| `config/service/hilton/otel` | an OTLP collector, or nothing | — | `OTEL_EXPORTER_OTLP_ENDPOINT`, `SERVICE`, `ENVIRONMENT`, `HOSTNAME` |
| `config/service/sql` | PostgreSQL | `DB_DRIVER`, `DB_DSN` | — |
| `config/service/redis` | Redis | `REDIS_HOST` | `REDIS_PORT` (`6379`), `REDIS_PASSWORD`, `REDIS_DB` (`0`) |
| `config/service/elastic` | Elasticsearch | `ES_HOST` | `ES_INDEX` |
| `config/tools/http` | — | `DAEMON_HOST`, `DAEMON_PORT` | `HEALTH_PATH` |
| `config/tools/grpc` | — | `DAEMON_HOST`, `DAEMON_PORT` | — |
| `config/tools/scribus/single`<br>`config/tools/scribus/batch` | RabbitMQ | `RABBIT_DSN`, `RABBIT_EXCHANGE`, `RABBIT_QUEUE` | — |
| `config/tools/migrate` | the database the migrations target | — | — |
| `config/tools/cron` | — | — | `CRON_SCHEDULE`, `TZ` (`UTC`) |

**Required** means `env.NeedValue` — the process stops at startup if the
variable is absent, before anything is served. **Optional** is `env.Value` with
the default shown in brackets; an empty default usually means the feature is
off, so no `SENTRY_DSN` means no Sentry channel and no
`OTEL_EXPORTER_OTLP_ENDPOINT` means spans go nowhere.

`config/tools/migrate` reads nothing itself: it runs against whatever
`config/service/sql` connected to.

## The minimum

A service that wires `config/app` and one transport needs two variables and no
infrastructure:

```
DAEMON_HOST=0.0.0.0
DAEMON_PORT=8080
```

Everything else on the list arrives with a module you chose.

## A worked example

`modules.go` for a service that serves HTTP, consumes a queue and has a
database:

```go
import (
    _ "gitlab.com/gorib/waffle/config/app"
    _ "gitlab.com/gorib/waffle/config/service/pry"
    _ "gitlab.com/gorib/waffle/config/service/sql"
    _ "gitlab.com/gorib/waffle/config/tools/http"
    _ "gitlab.com/gorib/waffle/config/tools/migrate"
    _ "gitlab.com/gorib/waffle/config/tools/scribus/single"
)
```

Running: PostgreSQL and RabbitMQ. `.env`:

```
DAEMON_HOST=0.0.0.0
DAEMON_PORT=8080

DB_DRIVER=pgx
DB_DSN=postgres://app:app@localhost:5432/app?sslmode=disable

RABBIT_DSN=amqp://guest:guest@localhost:5672/
RABBIT_EXCHANGE=events
RABBIT_QUEUE=my-service

LOGLEVEL=debug
SERVICE=my-service
ENVIRONMENT=local
```

How you start Postgres and RabbitMQ is your business — waffle has no opinion and
ships no compose file. What it does have is the list above, so that "it won't
start" is answered by reading rather than by asking.

## Two servers at once

`http` and `grpc` are two commands of one binary, so they share one address:
both read `DAEMON_HOST` and `DAEMON_PORT`. In a container that is what you want
— one process, one port, and the two commands run in different pods.

Locally, running both at the same time means the second one cannot bind. Both
commands take `--listen`, which wins over the wired address:

```sh
./app http
./app grpc --listen :9090
```

See [tools/http](../tools/http.md#listen-address) and
[tools/grpc](../tools/grpc.md#listen-address).

Enabling both modules does not duplicate the keys: `waffle` adds only what is
missing, so the second module contributes `HEALTH_PATH` and nothing else.

## When it does not start

| Symptom | Cause |
|---|---|
| exits immediately naming a variable | a `NeedValue` variable is missing from `.env` — find its module in the table |
| starts, then fails on the first query | `DB_DSN` points at a database that is not running or not migrated |
| consumer starts and receives nothing | the exchange or the binding does not exist yet; scribus declares the queue but never unbinds — see [tools/scribus](../tools/scribus.md) |
| no traces or metrics anywhere | `OTEL_EXPORTER_OTLP_ENDPOINT` is empty, which is the default — the provider is a no-op |
| logs are quiet | `LOGLEVEL` defaults to `info`; set `debug` locally |
