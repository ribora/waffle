# Context

[Docs](../README.md) · Core

Every command receives a `context.Context` populated with session metadata. This ensures that structured logging, tracing, and correlation IDs work consistently whether a request arrives over HTTP, gRPC, or the CLI.

## NewContextWithCommand

```go
func NewContextWithCommand(name string, args []string, provider hilton.Provider, logger pry.Logger) context.Context
```

Creates a `context.Background()` enriched with a `session.Session`:

| Session field | Value |
|--------------|-------|
| `Method` | `"CLI"` |
| `Uri` | Command name (e.g. `"migrate"`) |
| `Body` | Raw CLI arguments joined as a string |

The context is created in `Run()` before the command's `Run(ctx)` method is called. You do not call `NewContextWithCommand` directly.

It carries three more things, and they are the reason a command needs no constructor argument for any of them:

| Read it back with | What it is |
|---|---|
| `app.Args(ctx)` | the raw argv, so a command never touches `os.Args` — see [Args](#args) |
| `hilton.ProviderFromContext(ctx)` | the tracing and metrics provider, so an adapter instruments itself wherever the context reaches |
| `pry.LoggerFromContext(ctx)` | the logger, for code too far down to have been handed one |

The HTTP and gRPC middlewares put a session, the provider and the logger on a
request context, which is what makes a service package identical under a transport
and under the CLI.

## Args

```go
func Args(ctx context.Context) []string
func WithArgs(ctx context.Context, args []string) context.Context
```

`Args(ctx)` returns the argv shaped `[<command-name>, <rest>...]`, the form `(*getopt.Set).Getopt` accepts. Use it inside `Usage(ctx)` to parse flags:

```go
func (c *myCommand) Usage(ctx context.Context) error {
    opts := getopt.New()
    var help bool
    opts.FlagLong(&help, "help", 'h', "Display this help message")
    if err := opts.Getopt(app.Args(ctx), nil); err != nil {
        return err
    }
    // ...
}
```

This indirection exists so commands invoked by a scheduler (cron) see *their own* argv from the schedule entry, not the process's `os.Args`. The CLI dispatch path and the cron tick path both attach the right argv, so a command parses its flags the same way on either.

`WithArgs(ctx, args)` is the setter, used by orchestrators (cron) to build a child context for the command they're about to invoke. Most application code never calls it directly.

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
func (c *myCommand) Run(ctx context.Context) error {
    c.logger.Info("starting", pry.Ctx(ctx))
    // A Sentry event from this record includes Method=CLI and the command name as Uri.
    return c.work(ctx)
}
```

## HTTP and gRPC contexts

When a request arrives over HTTP or gRPC, the respective middleware populates the context with a session before it reaches your handler — `middleware.NewSession` and `interceptor.NewSessionMW`, with the logger and provider arriving from their own neighbours in the same chain. The session shape is the same, so `FromContext` works identically regardless of transport, and a service package cannot tell which one it is under.

## Next

| | |
|---|---|
| Parsing flags out of `app.Args(ctx)` | [core/commands](commands.md#flag-parsing-inside-a-command) |
| Who builds this context, and when | [core/application](application.md#run) |
| Why cron passes its own argv rather than the process's | [tools/cron](../tools/cron.md#how-a-tick-runs) |
| What the provider on the context is for | [tools/hilton](../tools/hilton.md#provider-lifecycle) |
