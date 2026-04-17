# Commands

Commands are the unit of work in a waffle application. Each runtime concern — serving HTTP, running migrations, consuming a queue — is a command. The app discovers all registered commands at startup and dispatches to the one named on the CLI.

## Interfaces

```go
type BaseCommand interface {
    Name() string
    Description() string
    Usage() error
}

type Command interface {
    BaseCommand
    Run(ctx context.Context) error
}
```

`BaseCommand` carries identity and help text. `Command` adds execution. The separation exists because tool constructors (e.g. `NewGrpcCommand`) accept a `BaseCommand` rather than a full `Command`, keeping the name/description wiring decoupled from the implementation.

## CommandList

`CommandList` is a `map[string]Command` with helper methods:

```go
type CommandList map[string]Command

func (c CommandList) Add(command Command) error
func (c CommandList) Get(command string) Command
func (c CommandList) List() <-chan Command
```

`Add` validates that the name is unique — duplicate registration returns an error. `List` returns commands over a channel in alphabetical order, used by the usage printer.

## Writing a custom command

Implement the `Command` interface. Embed `app.BaseCommand` (the interface) and store the result of `app.NewCommand` in it:

```go
type warmCacheCommand struct {
    app.BaseCommand
    cache *Cache
}

func NewWarmCacheCommand(cache *Cache) app.Command {
    return &warmCacheCommand{
        BaseCommand: app.NewCommand("warm-cache", "Pre-warm the Redis cache"),
        cache:       cache,
    }
}

func (c *warmCacheCommand) Run(ctx context.Context) error {
    if err := c.Usage(); err != nil {
        return err
    }
    return c.cache.Warm(ctx)
}
```

`Run` calls `Usage()` first to parse flags before execution begins.

## Registering a custom command

Add the registration inside `InitDi()` in `config/di/di.go`:

```go
di.Define(NewWarmCacheCommand, di.Tag(app.CommandTag))
```

After `make generate`, the app will list and dispatch `warm-cache` alongside the built-in commands.

## Flag parsing inside a command

Waffle uses `github.com/pborman/getopt/v2` for flag parsing. Implement `Usage() error` on your command, declare flags with `getopt` global functions, call `getopt.Parse()`, and handle `--help`:

```go
import (
    "os"
    "github.com/pborman/getopt/v2"
)

func (c *warmCacheCommand) Usage() error {
    var help bool
    getopt.FlagLong(&help, "help", 'h', "Display this help message")
    prefix := getopt.StringLong("prefix", 'p', "", "Key prefix to warm")
    getopt.Parse()
    if help {
        getopt.SetParameters("")
        os.Stderr.WriteString(c.Description() + "\n")
        getopt.PrintUsage(os.Stderr)
        os.Exit(2)
    }
    c.prefix = *prefix
    return nil
}
```

`getopt` is a global — all flags registered before `getopt.Parse()` is called are parsed together. There is no per-command flag set and no `FlagSet()` method on `BaseCommand`.

## Built-in commands

The following commands are provided by waffle tools. Copy their registrations into `InitDi()` in `config/di/di.go` — see each tool's documentation page for the exact block.

| Name | Package | Documentation |
|------|---------|---------------|
| `http` | `tools/http` | [tools/http.md](../tools/http.md) |
| `grpc` | `tools/grpc` | [tools/grpc.md](../tools/grpc.md) |
| `migrate` | `tools/migrate` | [tools/migrate.md](../tools/migrate.md) |
| `scribus` | `tools/scribus` | [tools/scribus.md](../tools/scribus.md) |
