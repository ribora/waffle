# Commands

[Docs](../README.md) · Core

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

A command that belongs to a domain is scaffolded. Do not write this one by hand:

```sh
waffle make:domain:command media sweep-otp
```

```
  + config/di/di_media_sweep_otp.go
  + domain/media/cmd/sweep-otp/main.go
```

The one operand decides every name — the CLI token `media:sweep-otp`, the
directory, the package `sweepotp`, `NewSweepOtpCommand`, `InitMediaSweepOtp` —
and the rules it has to satisfy are in [CLI overview](../cli/overview.md#command-names).
The command runs `make .tidy generate` itself, so the container has the
registration when it returns. What is left for you is the `Run` body and the
methods on the management interface the scaffold declared empty.

```go
// domain/media/cmd/sweep-otp/main.go
package sweepotp

type MediaManagement interface {
    // Methods consumed by this command go here.
}

func NewSweepOtpCommand(mediaManagement MediaManagement) *sweepOtpCommand {
    return &sweepOtpCommand{
        // Prefixed: a bare name can collide with a framework command, and app.Init fails.
        BaseCommand:     app.NewCommand("media:sweep-otp", "Sweep otp media"),
        mediaManagement: mediaManagement,
    }
}

type sweepOtpCommand struct {
    app.BaseCommand
    mediaManagement MediaManagement
}

func (c *sweepOtpCommand) Run(ctx context.Context) error {
    if err := c.Usage(ctx); err != nil {
        return err
    }
    // TODO: implement
    return nil
}
```

Three things in that shape are conventions, and the scaffold gets all three right.

**The token carries its domain.** The framework's own commands are bare words —
`http`, `grpc`, `migrate`, `cron`, `scribus`, `outbox` — in the same registry, and
a second command answering to one of them fails `app.Init`. That does not break
the one command; the binary refuses to start, taking every other command with it.

**`MediaManagement` is declared in the command's own package**, the same way a
transport declares what it asks of the domain. The command never imports the
service.

**`Run` calls `Usage(ctx)` first**, even with no flags of its own — that is what
makes `--help` work and what reads argv off the context rather than `os.Args`.

### By hand

`make:domain:command` takes a domain, so a command that belongs to no domain —
a one-off import, a maintenance task over several domains, anything at the level
of the service — is the case you write yourself. It is the same file without the
domain prefix in its name, which is exactly where the collision above becomes
your problem: pick a token no tool ships.

## Registering a custom command

The scaffold writes this file; you write it only for the by-hand case, and then
it goes in `config/di/di.go`:

```go
di.Define(sweepotp.NewSweepOtpCommand, di.Tag(app.CommandTag))
```

`di.Define`, because the constructor returns the unexported `*sweepOtpCommand`
and the tag collects it into `[]app.Command` regardless — a command is reached
through the registry by name, never as a dependency of something else, so there
is nothing here that wants an interface. The tool commands are the exception and
say why: they wire a marker interface (`di.Wire[http.HttpCommand](…)`) so that
`di.For[http.HttpCommand]()` can give that one command its own `BaseCommand`.

After `make generate` the app lists and dispatches `media:sweep-otp` alongside
the built-in commands. If it does not appear in `--help`, the container is stale.

## Flag parsing inside a command

Waffle uses `github.com/pborman/getopt/v2`. Each command parses its own argv from the context — never from `os.Args` directly — so cron and any other scheduler can invoke commands without race conditions on the global flag set.

The scaffold writes this method with `--help` and nothing else. A flag of your own is two lines in it — the declaration, and the field it lands in:

```go
import (
    "context"
    "fmt"
    "os"

    "github.com/pborman/getopt/v2"
    "github.com/ribora/waffle/app"
)

func (c *sweepOtpCommand) Usage(ctx context.Context) error {
    opts := getopt.New()
    var help bool
    opts.FlagLong(&help, "help", 'h', "Display this help message")
    olderThan := opts.StringLong("older-than", 'o', "24h", "Sweep media older than this")
    if err := opts.Getopt(app.Args(ctx), nil); err != nil {
        return err
    }
    if help {
        opts.SetParameters("")
        _, _ = fmt.Fprintf(os.Stderr, "%s\n", c.Description())
        opts.PrintUsage(os.Stderr)
        return app.ErrUsage
    }
    c.olderThan = *olderThan
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

This is the full set waffle ships. Each arrives with a blank import in
`config/di/modules.go` and with nothing else — no flag, no registry entry, no
line in `main.go` — so this table is also the list of modules a binary can
carry.

| Name | Daemon | Enable with | Read |
|---|---|---|---|
| `http` | yes | `_ ".../config/tools/http"` | [tools/http](../tools/http.md) |
| `grpc` | yes | `_ ".../config/tools/grpc"` | [tools/grpc](../tools/grpc.md) |
| `scribus` | yes | `_ ".../config/tools/scribus/<driver>"` **and** a mode, `.../single` or `.../batch` | [tools/scribus](../tools/scribus.md) |
| `outbox` | yes | `_ ".../config/tools/outbox/<driver>"` | [tools/outbox](../tools/outbox.md) |
| `cron` | no | `_ ".../config/tools/cron"` | [tools/cron](../tools/cron.md) |
| `migrate` | no | `_ ".../config/tools/migrate"` | [tools/migrate](../tools/migrate.md) |

The **Daemon** column is the `Daemon()` marker above, and cron refuses to
schedule anything in a `yes` row. The marker is on the command, not on the
subcommand, so `outbox status` and `outbox reset` return promptly and are still
refused — run those from a Kubernetes `CronJob` or by hand. `cron` itself is
long-running and unmarked: it is the scheduler, not something scheduled.

Two of them take a driver rather than naming a broker, which is why their import
line has a second segment. What each module needs running and in `.env` is
[reference/running-locally](../reference/running-locally.md#by-module); the
names that would collide with a domain command are [CLI
overview](../cli/overview.md#command-names).
