# Context

Every command receives a `context.Context` populated with session metadata. This ensures that structured logging, tracing, and correlation IDs work consistently whether a request arrives over HTTP, gRPC, or the CLI.

## NewContextWithCommand

```go
func NewContextWithCommand(name string, args []string) context.Context
```

Creates a `context.Background()` enriched with a `session.Session`:

| Session field | Value |
|--------------|-------|
| `Method` | `"CLI"` |
| `Uri` | Command name (e.g. `"migrate"`) |
| `Body` | Raw CLI arguments joined as a string |

The context is created in `Run()` before the command's `Run(ctx)` method is called. You do not call `NewContextWithCommand` directly.

## Reading session data

Use `session.FromContext` to access the session anywhere a context flows:

```go
import "gitlab.com/gorib/session"

func (c *myCommand) Run(ctx context.Context) error {
    s := session.FromContext(ctx)
    if s != nil {
        // s.Uri  → command name
        // s.Body → raw args
    }
    return nil
}
```

## Why this matters

`pry.Ctx(ctx)` reads the session from the context and attaches it to log events and Sentry reports. By storing session data in the context at the CLI boundary, all downstream calls — service layer, database adapters, HTTP clients — automatically carry the correlation context without any extra wiring.

```go
func (c *migrateCommand) Run(ctx context.Context) error {
    c.logger.Info("starting migration", pry.Ctx(ctx))
    // Sentry event will include Method=CLI, Uri=migrate
    return c.runMigrations(ctx)
}
```

## HTTP and gRPC contexts

When a request arrives over HTTP or gRPC, the respective middleware populates the context with a session before it reaches your handler. The session shape is the same — `FromContext` works identically regardless of transport.
