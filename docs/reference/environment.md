# Environment Variables

All environment variables are read using `gitlab.com/gorib/env`, which loads a `.env` file from the working directory at startup and falls back to the process environment.

A variable is only read if the module that reads it is enabled — every one below comes from a blank import in `config/di/modules.go`. `NeedValue` panics at startup when the variable is missing, so a misconfigured service fails immediately rather than at the first request; `Value` takes a fallback.

---

## Service modules (`config/service/…`)

### `config/service/sql`

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DB_DRIVER` | yes | — | `postgres`, `mysql` or `sqlite` |
| `DB_DSN` | yes | — | Connection string, e.g. `postgres://user:pass@host:port/dbname?sslmode=disable` |

### `config/service/redis`

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `REDIS_HOST` | yes | — | Hostname |
| `REDIS_PORT` | no | `6379` | Port |
| `REDIS_PASSWORD` | no | empty | Password |
| `REDIS_DB` | no | `0` | Database index |

### `config/service/elastic`

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ES_HOST` | yes | — | Cluster address |
| `ES_INDEX` | no | empty | Prefix prepended to every index name |

### `config/service/pry`

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `LOGLEVEL` | no | `info` | `trace`, `debug`, `info`, `warn` or `error` |
| `SENTRY_DSN` | no | empty | Empty disables the Sentry channel without disabling the logger |
| `SENTRY_LEVEL` | no | `error` | What reaches Sentry. Cannot be lower than `LOGLEVEL`: the level gate runs before the record is built, so a record the logger never emits cannot reach a sink |
| `ENVIRONMENT` | no | `noenv` | Reported to Sentry |
| `SERVICE` | no | `nocontour` | Reported to Sentry as the server name |

### `config/service/hilton/otel`

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | no | empty | Empty means no exporter — the provider becomes a no-op |
| `SERVICE` | no | empty | Resource service name |
| `ENVIRONMENT` | no | empty | Resource deployment environment |
| `HOSTNAME` | no | empty | Resource instance id; set by the container runtime in practice |

---

## Tool modules (`config/tools/…`)

### `config/tools/http` and `config/tools/grpc`

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DAEMON_HOST` | yes | — | Listen address |
| `DAEMON_PORT` | yes | — | Listen port |
| `HEALTH_PATH` | no | empty | `http` only — empty means the middleware's own default path |

Both servers read the same pair, so a service running HTTP and gRPC in one process runs them as two commands, not two listeners in one.

### `config/tools/scribus`

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `RABBIT_DSN` | yes | — | `amqp://user:pass@host:port/` |
| `RABBIT_EXCHANGE` | yes | — | Exchange to bind the queue to |
| `RABBIT_QUEUE` | yes | — | Queue name |

Page size and flush interval are wired as constants in the module, not read from the environment; both can be overridden per run with `--page` and `--flush-timeout`. See [tools/scribus.md](../tools/scribus.md).

### `config/service/amqp`

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `RABBIT_DSN` | yes | — | `amqp://user:pass@host:port/` |
| `RABBIT_EXCHANGE` | yes | — | Exchange the publisher writes to |

The one place a publisher is built, so a service that also enables the outbox gets
one instance on one connection rather than two.

### `config/tools/outbox`

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OUTBOX_MAX_ATTEMPTS` | no | `10` | Failures before an entry is left for `outbox reset` |
| `OUTBOX_RELAY_NAME` | no | the hostname | Written into `claimed_by`, so a stuck claim names its pod |
| `OUTBOX_POLL_INTERVAL` | no | `5s` | The longest pause between passes when the outbox is empty |
| `OUTBOX_STALE_AFTER` | no | `1m` | How long a claim is honoured, and so how soon a failure is retried; under a second is raised to a second |
| `OUTBOX_RETENTION` | no | `168h` | How long delivered entries are kept before the relay purges them; zero or less never purges |
| `OUTBOX_PURGE_INTERVAL` | no | `1h` | How often the relay looks for delivered entries to purge |

`DB_DRIVER` and `DB_DSN` come with `config/service/sql`, which this one pulls in.
The broker's variables belong to the driver beside it — `RABBIT_DSN` and
`RABBIT_EXCHANGE` for `config/tools/outbox/amqp`, which reads them through
`config/service/amqp`. See [tools/outbox.md](../tools/outbox.md).

### `config/tools/cron`

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `CRON_SCHEDULE` | no | empty | Schedule definition; empty means no jobs |
| `TZ` | no | `UTC` | Timezone the schedule is interpreted in |

---

## Migrations

The migrate command uses the `sql.Db` from the container — that is, `DB_DRIVER` and `DB_DSN` — and accepts a `--connection` flag to override it at runtime:

```sh
./app migrate --connection "postgres://admin:secret@prod-host:5432/mydb" up
```

The flag's scheme selects the driver, so it replaces both variables at once.

---

## Example `.env`

```
DAEMON_HOST=0.0.0.0
DAEMON_PORT=8080

DB_DRIVER=postgres
DB_DSN=postgres://user:pass@localhost:5432/mydb?sslmode=disable

REDIS_HOST=localhost

RABBIT_DSN=amqp://guest:guest@localhost:5672/
RABBIT_EXCHANGE=events
RABBIT_QUEUE=my-service

LOGLEVEL=debug
SERVICE=my-service
ENVIRONMENT=local
```

---

## Adding your own variables

Use `gitlab.com/gorib/env` in `config/config.go`:

```go
package config

import (
    "time"
    "gitlab.com/gorib/env"
)

type Config struct {
    JWTSecret      string
    CacheAddr      string
    RequestTimeout time.Duration
}

func NewConfig() *Config {
    return &Config{
        JWTSecret:      env.NeedValue[string]("JWT_SECRET"),
        CacheAddr:      env.Value("CACHE_ADDR", "localhost:6379"),
        RequestTimeout: env.Value("REQUEST_TIMEOUT", 30*time.Second),
    }
}
```

See the [env package documentation](https://gitlab.com/gorib/env) for the full API reference.
