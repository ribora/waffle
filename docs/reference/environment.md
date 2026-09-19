# Environment Variables

[Docs](../README.md) · Reference

All environment variables are read using `gitlab.com/gorib/env`, which loads a `.env` file from the working directory at startup and falls back to the process environment.

A variable is only read if the module that reads it is enabled — every one below comes from a blank import in `config/di/modules.go`. `NeedValue` panics at startup when the variable is missing, so a misconfigured service fails immediately rather than at the first request; `Value` takes a fallback.

---

## Service modules (`config/service/…`)

### `config/service/sql`

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DB_DRIVER` | yes | — | `pgx` is what `init:service` writes. See below |
| `DB_DSN` | yes | — | Connection string, e.g. `postgres://user:pass@host:port/dbname?sslmode=disable` |

`postgres`, `pgx` and `pgsql` all select the native pgx pool — the scaffolded
`.env` says `pgx`. Any other value is passed to `database/sql` as a driver name,
so it has to be one your binary registers by import: `mysql`, `sqlite`,
`sqlite3`. The same string is the goose dialect when `config/tools/migrate` is
enabled, which is the reason the two modules share one variable rather than each
reading its own.

### `config/service/redis`

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `REDIS_HOST` | yes | — | Hostname |
| `REDIS_PORT` | no | `6379` | Port |
| `REDIS_PASSWORD` | no | empty | Password |
| `REDIS_DB` | no | `0` | Database index |

### `config/service/amqp`

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `RABBIT_DSN` | yes | — | `amqp://user:pass@host:port/` |
| `RABBIT_EXCHANGE` | yes | — | Exchange the publisher writes to |

The one place a publisher is built, so a service that also enables the outbox gets
one instance on one connection rather than two.

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

### `config/tools/scribus/amqp`

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `RABBIT_DSN` | yes | — | `amqp://user:pass@host:port/` |
| `RABBIT_EXCHANGE` | yes | — | Exchange to bind the queue to |
| `RABBIT_QUEUE` | yes | — | Queue name |

The **driver** module, not `config/tools/scribus` itself: the command names no broker, so a driver of your own reads whatever it likes and none of these three exist. Page size and flush interval are wired as constants there rather than read from the environment; both can be overridden per run with `--page` and `--flush-timeout`. See [tools/scribus.md](../tools/scribus.md).

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

DB_DRIVER=pgx
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

**There is no `Config` struct**, and `config/config.go` is not where this goes —
that file is the three-line wrapper around `diogen.InitDi()` and stays that way.
A variable is read where the thing that needs it is registered, in
`config/di/di.go`, the same way every module above does it:

```go
// config/di/di.go
func InitDi() {
    di.Wire[auth.Signer](auth.NewSigner, di.Args(map[string]any{
        "secret": env.NeedValue[string]("JWT_SECRET"),
        "ttl":    env.Value("JWT_TTL", 15*time.Minute),
    }))
}
```

`di.Args` values are evaluated at container build time, so a missing
`NeedValue` stops the process at startup with the variable's name — the same
failure the framework's own modules give, rather than a nil field discovered on
the first request.

That the reads sit next to the registration is also what `waffle agents` walks
to fill in the env list in `AGENTS.md`: it greps `env.Value` / `env.NeedValue`
across `config/`, `domain/` and `cmd/`. A struct built somewhere else is
invisible to it.

Two rules follow, and they are the reason there is no central config object:

- **Nothing reads the environment below the composition root.** A service or an
  adapter takes the value as a constructor argument. That is what makes it
  replaceable in a component test, which sets no environment at all.
- **A variable arrives with the thing that reads it**, so it goes into `.env`
  in the same change — the convention every scaffolding command follows.

See the [env package documentation](https://gitlab.com/gorib/env) for the full API reference.
