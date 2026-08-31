# Environment Variables

All environment variables are read using `gitlab.com/gorib/env`, which loads a `.env` file from the working directory at startup and falls back to the process environment.

---

## RabbitMQ (scribus tool)

Environment variables for the scribus consumer are wired explicitly in your `InitDi()`. Variable names are chosen by you — see [tools/scribus.md](../tools/scribus.md) for the wiring example.

### Example `.env`

```
RABBIT_DSN=amqp://guest:guest@localhost:5672/
RABBIT_EXCHANGE=events
RABBIT_QUEUE=my-service
RABBIT_DLX_EXCHANGE=errors
```

---

## Database

Read by your `config/config.go` — waffle does not read database configuration itself. The conventional variable names are:

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL DSN. Format: `postgres://user:pass@host:port/dbname?sslmode=disable` |

The migrate command accepts a `--connection` flag to override the DSN at runtime:

```sh
./app migrate --connection "postgres://admin:secret@prod-host:5432/mydb" up
```

---

## Application

No application-level environment variables are read by waffle itself. Service-specific configuration is the responsibility of your `config/config.go`.

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
    DatabaseURL    string
    JWTSecret      string
    CacheAddr      string
    RequestTimeout time.Duration
}

func NewConfig() *Config {
    return &Config{
        DatabaseURL:    env.NeedValue[string]("DATABASE_URL"),
        JWTSecret:      env.NeedValue[string]("JWT_SECRET"),
        CacheAddr:      env.Value("CACHE_ADDR", "localhost:6379"),
        RequestTimeout: env.Value("REQUEST_TIMEOUT", 30*time.Second),
    }
}
```

`NeedValue` panics at startup if the variable is missing, which surfaces misconfiguration immediately rather than at the first request. `Value` accepts a fallback for optional settings.

See the [env package documentation](https://gitlab.com/gorib/env) for the full API reference.
