# Cron Tool

[Docs](../README.md) · Tools

The cron tool runs other waffle commands on a schedule. It is itself a `Command` — invoking `./app cron` starts a long-running scheduler that fires registered commands according to a crontab-style configuration. Each tick re-enters the target command's `Run(ctx)` with the argv specified in the schedule.

## Setup

Add a single blank import to `config/di/modules.go`:

```go
// config/di/modules.go
package di

import (
    _ "github.com/ribora/waffle/config/tools/cron"
    // ... other modules ...
)
```

This pulls in the registrations from [`config/tools/cron/init.go`](../../config/tools/cron/init.go), equivalent to:

```go
import (
    "github.com/ribora/diogen/di"
    "github.com/ribora/env"
    "github.com/ribora/waffle/app"
    "github.com/ribora/waffle/tools/cron"
)

di.Wire[app.BaseCommand](
    func() app.BaseCommand { return app.NewCommand("cron", "Run scheduled commands") },
    di.For[cron.CronCommand](),
)
di.Wire[cron.CronCommand](cron.NewCronCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
    "schedule": env.Value("CRON_SCHEDULE", ""),
    "timezone": env.Value("TZ", "UTC"),
}))
```

Run `make generate`, then start the scheduler:

```sh
./app cron
```

With an empty `CRON_SCHEDULE` the command exits immediately.

## Schedule format

`CRON_SCHEDULE` is a newline-separated list of crontab-style entries:

```
<minute> <hour> <day-of-month> <month> <day-of-week>  <command-name> [args...]
```

The first five whitespace-separated tokens are the standard crontab expression. Everything after them is the target command's argv — split on whitespace, no quoting.

```
CRON_SCHEDULE="*/15 * * * *  rent lock-unpaid
0 8,20 * * *    rent renew
0 9 * * 1       migrate status"
```

Lines starting with `#` and blank lines are ignored.

**Why newlines, not commas.** Crontab natively uses commas as a list-of-values separator (`0 8,20 * * *` means "8:00 and 20:00"). The env helper `env.Array` splits on commas, which would collide with cron syntax, so cron deliberately reads `CRON_SCHEDULE` as a single string and splits on newlines internally.

## How a tick runs

For each entry:

1. gocron fires on the schedule.
2. Cron looks up the target command by `argv[0]` in the waffle command registry.
3. A child context is built with `Method=CRON`, `Uri=<command-name>`, `Body=<argv joined>`, and argv attached via `app.WithArgs`.
4. The target command's `Run(ctx)` is invoked. Inside, `app.Args(ctx)` returns the schedule entry's argv — `getopt` sees exactly what the schedule line specifies, never the host process's `os.Args`.

Each entry runs in singleton mode (`gocron.LimitModeWait`): overlapping ticks for the same entry queue rather than run in parallel. Different entries run independently.

## Daemons are refused

HTTP/gRPC servers, queue consumers, and other long-running commands implement the `app.Daemon` marker interface. Cron validates the schedule at startup and refuses any entry pointing at a daemon:

```
$ ./app cron
cron: command is a daemon, cannot schedule: http
```

Daemons block in their `Run` until ctx is cancelled. Putting one on a tick would jam the scheduler. The check is fail-fast at startup — not on first fire, where you might not notice until hours later.

See [Commands → Daemons](../core/commands.md#daemons) for how to mark your own long-running commands.

## Unknown commands

Schedule entries that reference an unregistered command name are rejected at startup:

```
$ ./app cron
cron: unknown command in schedule: rebt
```

## Timezone

`TZ` env var controls when cron expressions fire. Standard IANA names (`Europe/Berlin`, `America/Los_Angeles`, `UTC`). Default is `UTC`.

```
TZ=Europe/Berlin
CRON_SCHEDULE="0 9 * * 1-5  reports daily"
```

The schedule above fires at 09:00 Berlin time on weekdays.

## Logging

Failed task runs are logged via the bound `pry.Logger` with `Method=CRON` in the session, so log lines and Sentry events show which command (and which argv) failed. Successful runs are silent at every log level: nothing logs a tick.

## Running a scheduled command manually

To invoke a cron-scheduled command once for debugging, run it directly:

```sh
./app rent lock-unpaid
```

This is equivalent to one cron fire of the corresponding entry — the command sees the same context shape (minus `Method=CRON` and singleton coordination).

## Architecture

Cron is just another `Command` tagged `app.CommandTag`. The only special wiring is `app.CommandsAware`: the cron command implements `SetCommands(app.Commands)`, and `app.Init` injects the peer registry once after building it. Cron stores the read-only view and dispatches through `commands.Get(name)` on each tick.

No special-casing in `app.Init` for cron — `CommandsAware` is a generic mechanism. Any future orchestrator can reuse it.

## Limitations

- **No per-tick cancellation propagation** beyond gocron's shutdown signal. If you need a per-tick timeout, wrap your target command's `Run` with `context.WithTimeout` internally.
- **No retry semantics.** A failed tick is logged and skipped; the next scheduled tick fires normally. Use the target command's own error handling for retry policies.
- **No `@reboot` or `@daily`-style aliases.** Standard 5-field crontab only. `@reboot` is a system-cron concept that does not map to container lifecycle anyway.
- **No second-precision schedules.** 5-field minute-precision only.

## Next

| | |
|---|---|
| Writing a command worth scheduling | [core/commands](../core/commands.md#writing-a-custom-command) |
| Scaffolding one scoped to a domain | `waffle make:domain:command <domain> <command>` — [cli/overview](../cli/overview.md#command-names) |
| Marking your own long-running command a daemon | [core/commands](../core/commands.md#daemons) |
| What the tick's context carries, and why not `os.Args` | [core/context](../core/context.md#args) |
