# Commands

Commands are the unit of work in a waffle application. Each runtime concern — serving HTTP, running migrations, consuming a queue — is a command. The app discovers all registered commands at startup and dispatches to the one named on the CLI.

## Interfaces

```go
type BaseCommand interface {
    Name() string
    Description() string
    Usage(ctx context.Context) error
}

type Command interface {
    BaseCommand
    Run(ctx context.Context) error
}
```

`BaseCommand` carries identity and help text. `Command` adds execution. The separation exists because tool constructors (e.g. `NewGrpcCommand`) accept a `BaseCommand` rather than a full `Command`, keeping the name/description wiring decoupled from the implementation.

`Usage(ctx)` reads argv from the context (see [Context](context.md)) and parses flags. It is called by `Run` before the command starts work.

## Writing a custom command

Implement the `Command` interface. Embed `app.BaseCommand` (the interface) and store the result of `app.NewCommand` in it:

```go
type warmCacheCommand struct {
    app.BaseCommand
    cache  *Cache
    prefix string
}

func NewWarmCacheCommand(cache *Cache) app.Command {
    return &warmCacheCommand{
        BaseCommand: app.NewCommand("warm-cache", "Pre-warm the Redis cache"),
        cache:       cache,
    }
}

func (c *warmCacheCommand) Run(ctx context.Context) error {
    if err := c.Usage(ctx); err != nil {
        return err
    }
    return c.cache.Warm(ctx, c.prefix)
}
```

## Registering a custom command

Add the registration inside `InitDi()` in `config/di/di.go`:

```go
di.Define(NewWarmCacheCommand, di.Tag(app.CommandTag))
```

After `make generate`, the app will list and dispatch `warm-cache` alongside the built-in commands.

## Flag parsing inside a command

Waffle uses `github.com/pborman/getopt/v2`. Each command parses its own argv from the context — never from `os.Args` directly — so cron and any other scheduler can invoke commands without race conditions on the global flag set.

```go
import (
    "context"
    "fmt"
    "os"

    "github.com/pborman/getopt/v2"
    "gitlab.com/gorib/waffle/app"
)

func (c *warmCacheCommand) Usage(ctx context.Context) error {
    opts := getopt.New()
    var help bool
    opts.FlagLong(&help, "help", 'h', "Display this help message")
    prefix := opts.StringLong("prefix", 'p', "", "Key prefix to warm")
    if err := opts.Getopt(app.Args(ctx), nil); err != nil {
        return err
    }
    if help {
        opts.SetParameters("")
        _, _ = fmt.Fprintf(os.Stderr, "%s\n", c.Description())
        opts.PrintUsage(os.Stderr)
        return app.ErrUsage
    }
    c.prefix = *prefix
    return nil
}
```

The pattern is always:

1. `opts := getopt.New()` — a fresh option set, never the global default.
2. Declare flags on `opts`.
3. `opts.Getopt(app.Args(ctx), nil)` — parse the context-borne argv.
4. Handle `--help` by returning `app.ErrUsage`.

Returning `app.ErrUsage` makes the process exit with code 2 (POSIX convention for usage errors) without logging it as an error.

## Daemons

Long-running commands — HTTP/gRPC servers, queue consumers — implement the `Daemon` marker interface:

```go
type Daemon interface {
    Command
    Daemon()
}
```

The implementation is a no-op method:

```go
func (c *httpCommand) Daemon() {}
```

Schedulers like cron (see [tools/cron](../tools/cron.md)) refuse to invoke daemons because their `Run(ctx)` blocks until ctx is cancelled — invoking one on a cron tick would jam the scheduler forever. Mark any command whose `Run` does not return promptly.

## Orchestrators and CommandsAware

Commands that need peer access (e.g. cron, which dispatches other commands on a schedule) implement `CommandsAware`:

```go
type Commands interface {
    Get(name string) Command
}

type CommandsAware interface {
    SetCommands(Commands)
}
```

`app.Init` calls `SetCommands` once after the registry is built, before the app starts dispatching. The `Commands` view is read-only by design — orchestrators look up peers by name but cannot mutate the registry.

Cron is the only built-in `CommandsAware` consumer. Custom orchestrators are possible but rare; most cross-cutting concerns belong in middleware, not in a sibling command.

## Built-in commands

The following commands are provided by waffle tools. Include each tool's `Init...()` in your `modules.go` to register the corresponding command.

| Name | Package | Documentation |
|------|---------|---------------|
| `http` | `tools/http` | [tools/http.md](../tools/http.md) |
| `grpc` | `tools/grpc` | [tools/grpc.md](../tools/grpc.md) |
| `migrate` | `tools/migrate` | [tools/migrate.md](../tools/migrate.md) |
| `scribus` | `tools/scribus` | [tools/scribus.md](../tools/scribus.md) |
| `cron` | `tools/cron` | [tools/cron.md](../tools/cron.md) |
