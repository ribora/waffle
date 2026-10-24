# Scribus Tool

[Docs](../README.md) · Tools

The scribus tool turns a message consumer into a waffle `Command`. It supports two modes: **single mode**, where each subject is handled by a dedicated `Handler`, and **batch mode**, where a single `BatchHandler` processes all messages together.

The tool itself names no broker: a handler receives a `whisk/scan.Source`, and the routing semantics come from a driver. The one that ships is `config/tools/scribus/amqp`, over `gitlab.com/gorib/amqp`.

## Setup

A service includes **one driver** and **one mode**. The driver brings the consumer, its `Subjects` and its middlewares; the mode brings the dispatch shape.

`waffle make:domain:scribus` enables both, and takes the driver as a required flag — `--driver amqp` — because picking a broker for you is the one thing this tool avoids. It refuses before writing anything, naming the line or the file, if:

- `config/di/modules.go` has no line for the driver or the mode;
- the other mode is already enabled — `single` and `batch` both register a command named `scribus`, and the binary stops starting;
- another driver is already enabled — a service consumes through one;
- with `--batch`, `config/di` already wires a `BatchHandler` — `NewBatchScribusCommand` takes exactly one, so the next subject goes into its `Keys()`.

**Single mode** (each subject dispatched to a dedicated `Handler`):

```go
// config/di/modules.go
package di

import (
    _ "gitlab.com/gorib/waffle/config/tools/scribus/amqp"
    _ "gitlab.com/gorib/waffle/config/tools/scribus/single"
    // ... other modules ...
)
```

This expands to the registrations from [`config/tools/scribus/init.go`](../../config/tools/scribus/init.go) (common), [`config/tools/scribus/amqp/init.go`](../../config/tools/scribus/amqp/init.go) (driver) and [`config/tools/scribus/single/init.go`](../../config/tools/scribus/single/init.go):

```go
// common — from config/tools/scribus
di.Wire[app.BaseCommand](
    func() app.BaseCommand { return app.NewCommand("scribus", "Start the event consumer") },
    di.For[wscribus.ScribusCommand](),
)

// driver — from config/tools/scribus/amqp
di.Wire[wscribus.Consumer](amqp.NewConsumer, di.Args(map[string]any{
    "dsn":      env.NeedValue[string]("RABBIT_DSN"),
    "exchange": env.NeedValue[string]("RABBIT_EXCHANGE"),
    "queue":    env.NeedValue[string]("RABBIT_QUEUE"),
    "opts": []amqp.ConsumerOptions{
        amqp.WithConsumerPageSize(1),
        amqp.WithConsumerFlushInterval(5 * time.Second),
    },
}))
di.Wire[wscribus.Subjects](amqp.Subjects)
tracing := di.Define(interceptor.NewTracing)
logging := di.Define(interceptor.NewLog)
recovering := di.Define(interceptor.NewRecover)

di.Wire[wscribus.Middlewares](wscribus.NewMiddlewares, di.Fallback(), di.Args(map[string]any{
    "middlewares": di.List(tracing, logging, recovering),
}))

// single-mode — from config/tools/scribus/single
di.Wire[wscribus.ScribusCommand](wscribus.NewScribusCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
    "commands": di.Tags(wscribus.CommandTag),
}))
```

**Batch mode** (one `BatchHandler` processes all messages):

```go
// config/di/modules.go
import (
    _ "gitlab.com/gorib/waffle/config/tools/scribus/amqp"
    _ "gitlab.com/gorib/waffle/config/tools/scribus/batch"
)

// config/di/di.go — register your own BatchHandler:
func InitDi() {
    di.Wire[wscribus.BatchHandler](scribusCommand.NewNotificationAcceptedCommand)
}
```

`config/tools/scribus/batch` pulls in the same common block plus `di.Wire[ScribusCommand](NewBatchScribusCommand, ...)`. Your `BatchHandler` registration stays in your own `InitDi()`.

Run `make generate` after adding this.

## Running the consumer

```sh
./app scribus
```

The connection operands are the driver's, not the command's: it asks the consumer
for their names through `Transport.Params()`, prints them in `--help`, and passes
the values back in that order. The AMQP driver answers `dsn exchange queue`, so
`./app scribus --help` prints `[dsn exchange queue]` and a full set overrides what
DI wired — useful for ad-hoc local debugging against a different broker:

```sh
./app scribus amqp://guest:guest@localhost:5672 events local-debug
```

With no operands the values come from DI, which for that driver means `RABBIT_DSN`,
`RABBIT_EXCHANGE` and `RABBIT_QUEUE`. A partial set is refused, naming the operands
the driver takes rather than any fixed three.

Subjects are **not** an argument of the running command — they come from each registered `Handler.Keys()` / `BatchHandler.Keys()`, and this CLI only overrides the connection, never the bindings. The scaffolder does take one, as the value it writes into that `Keys()`: `waffle make:domain:scribus --driver amqp user order_paid order.paid`.

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--flush-timeout` | `-t` | `0` (use default) | Flush every [n] seconds |
| `--page` | `-p` | `0` (use default) | Messages per page; above 1 requires batch mode — see [Page size and settlement](#page-size-and-settlement) |
| `--reconnect-timeout` | `-r` | `0` (use default) | Reconnect every [n] seconds |

## Environment variables

The command itself reads none. The **driver** does, in the registration shown in
Setup above — `config/tools/scribus/amqp` reads `RABBIT_DSN`,
`RABBIT_EXCHANGE` and `RABBIT_QUEUE`, all three `env.NeedValue`, so a missing one
stops the process at startup. A driver of your own names whatever it likes;
`scribus` never sees a variable. Full list in
[reference/environment](../reference/environment.md#configtoolsscribusamqp).

The CLI operands take precedence over all of it at runtime — see [Running the
consumer](#running-the-consumer).

## Subjects

Subjects are **not** configured in env. Each `Handler` (single mode) or `BatchHandler` (batch mode) declares its own via `Keys() []string`, and scribus binds the consumer to them at startup. Adding a new handler — or a new subject on an existing handler — automatically extends the binding set on next deploy.

Bindings are additive: `QueueBind` is idempotent at the broker level, so repeated startups don't churn. Removing a subject from a `Handler` stops new messages from being delivered for it only after the stale binding is removed manually via RabbitMQ Management UI (or `rabbitmqadmin delete binding`). Scribus does not unbind for you — the consumer code only knows what it currently *wants*, not what was bound before.

For ad-hoc fan-out (consume everything), implement `Keys() []string{"#"}` on the handler. Wildcard semantics belong to the driver's `Subjects`, not to scribus — under the AMQP one they are RabbitMQ's.

## Single mode

In single mode, each subject dispatches to its own `Handler`.

### Handler interface

```go
type Handler interface {
    Keys() []string
    Run(src scan.Source) error
}
```

`Keys()` returns the subjects this handler subscribes to. `Run` is called for each incoming message with a `scan.Source` — body, context, and everything else through `Lookup`. Parse it with `scan.Into[T](src)`. Under the AMQP driver the value is an `amqp.Source`, so a handler that needs the exchange can assert for it and say so.

### Message model

Define a local struct for the message payload in `cmd/scribus/model/`. It is separate from the domain model and can have `json` tags:

```go
// cmd/scribus/model/order.go

//go:generate go tool whisk $GOFILE

package model

type OrderCreated struct {
    Id         int         `json:"id"`
    CustomerId int         `json:"customerId"`
    Items      []OrderItem `json:"items"`
}
```

`scan.Into` needs the `Parse` method whisk generates, so the message model carries the same `//go:generate` directive as an HTTP request DTO — it is the same mechanism over a different source. AMQP contributes the scopes `header`, `routing-key` and `exchange`; validation tags, hooks and custom validators are described in [conventions/input-parsing.md](../conventions/input-parsing.md).

### Implementing a handler

The handlers of a domain consume one management interface, declared beside them (consumer-defined, same pattern as gRPC). `make:domain:scribus` writes it once per domain, in `cmd/scribus/management.go`, and wires it once, in `config/di/di_<domain>_scribus.go`; the next handler adds its methods to the same interface.

```go
// cmd/scribus/management.go
package scribus

type OrderManagement interface {
    HandleOrderCreated(ctx context.Context, order *model.Order) error
}
```

The struct is private; the constructor is public.

```go
// cmd/scribus/order_created.go
package scribus

import (
    "context"

    scribusModel "mymodule/domain/order/cmd/scribus/model"
    "mymodule/domain/order/model"
    "gitlab.com/gorib/pry"
    "gitlab.com/gorib/whisk/scan"
)

func NewOrderCreatedHandler(management OrderManagement, logger pry.Logger) *orderCreatedHandler {
    return &orderCreatedHandler{management: management, logger: logger}
}

type orderCreatedHandler struct {
    management OrderManagement
    logger     pry.Logger
}

func (h *orderCreatedHandler) Keys() []string {
    return []string{"order.created"}
}

func (h *orderCreatedHandler) Run(src scan.Source) error {
    msg, err := scan.Into[scribusModel.OrderCreated](src)
    if err != nil {
        h.logger.Error("Cannot parse message", pry.Err(err), pry.Ctx(src.Context()))
        return nil // ACK: malformed message, re-queuing won't help
    }

    err = h.management.HandleOrderCreated(src.Context(), &model.Order{
        Id:         msg.Id,
        CustomerId: msg.CustomerId,
    })
    if err != nil {
        h.logger.Error("Cannot handle order.created", pry.Err(err), pry.Ctx(src.Context()))
    }
    return nil // ACK: log the error and move on
}
```

**Error handling**: scribus handlers always return `nil`. If processing fails, log the error and acknowledge the message. Returning a non-nil error causes the message to be re-queued, which is rarely the right behaviour for event-driven notifications.

### Registering a handler

Register with `di.Define` and tag with `wscribus.CommandTag`. A handler is reached only through the tag, so it needs no interface binding:

```go
di.Define(scribus.NewOrderCreatedHandler, di.Tag(wscribus.CommandTag))
```

Wire the service dependency via `di.Alias` on the service registration:

```go
di.Define(service.NewOrderService,
    di.Alias[scribus.OrderManagement](),
)
```

## Batch mode

In batch mode, a single `BatchHandler` receives a slice of messages accumulated over time (by page size or flush interval).

### BatchHandler interface

```go
type BatchHandler interface {
    Keys() []string
    Run(msgs []scan.Source) error
}
```

`Keys()` returns the subjects this batch handler subscribes to — scribus auto-binds them on the queue at startup. `Run` is called with a slice of `scan.Source`; each carries its own body, context and scopes.

### Implementing a batch handler

```go
type orderBatchHandler struct {
    management OrderManagement
    logger     pry.Logger
}

func NewOrderBatchHandler(management OrderManagement, logger pry.Logger) *orderBatchHandler {
    return &orderBatchHandler{management: management, logger: logger}
}

func (h *orderBatchHandler) Run(msgs []scan.Source) error {
    var (
        skipped int
        cause   error
    )
    for _, m := range msgs {
        msg, err := scan.Into[scribusModel.OrderCreated](m)
        if err != nil {
            skipped++
            if cause == nil {
                cause = err
            }
            continue
        }
        if err = h.management.HandleOrderCreated(m.Context(), &model.Order{Id: msg.Id}); err != nil {
            h.logger.Error("Cannot handle "+m.Lookup("routing-key", ""), pry.Err(err), pry.Ctx(m.Context()))
        }
    }
    if skipped > 0 {
        h.logger.Error("Skipped unparsable events",
            pry.Err(cause), pry.Field("count", skipped), pry.Ctx(msgs[len(msgs)-1].Context()))
    }
    return nil
}
```

A page is as large as `--page` says, so a per-message line turns one bad publisher
into five hundred log entries. Count them and log once. Keep the payload out of it
either way: these are invoices and contracts, and a log line is read by more people
than the database is.

Register exactly one `BatchHandler` without a tag:

```go
di.Wire[wscribus.BatchHandler](scribus.NewOrderBatchHandler)
```

## How routing works

On startup the scribus command:

1. Collects every registered handler's `Keys()` and calls `consumer.Bind(subject)` for each — the broker is told to deliver only those subjects.
2. (Optional) overrides DSN/exchange/queue from CLI positional args.
3. Opens the consumer; for each incoming message reads its subject through the driver's `Subjects.Of`.
4. Accumulates messages until `--page` is reached or `--flush-timeout` expires — both modes go through the same accumulator.
5. In single mode: for each message in the page, looks up the handler whose subject `Subjects.Match` accepts and calls `Run`. In batch mode: calls `BatchHandler.Run` once with the whole page.

## Page size and settlement

A page is acknowledged as one unit: `Ack`/`Nack` carry AMQP's `multiple` flag as soon as it holds more than one message. So the page size, not the mode, decides how much is at stake per failure — and one failure nacks the page with requeue.

`config/tools/scribus` pins the size to 1 for both modes, so out of the box every message settles on its own. `--page` raises it, and the two modes treat that differently:

| | `--page` unset or 1 | `--page` above 1 |
|---|---|---|
| **single** | per-message settlement, which is the point of the mode | **refused at startup** — the handler is called per message but the page settles together, so one failure requeues its neighbours and skips the rest |
| **batch** | legal, warns: the handler is handed one message at a time | the intended configuration |

The refusal covers the flag. A service that registers its own `scribus.Consumer` with a larger page size bypasses it, because by then nothing knows which mode was wired.

## DI tags

| Tag | Collects |
|-----|---------|
| `scribus.CommandTag` | All `Handler` implementations (single mode) |

`BatchHandler` is provided without a tag — only one may be registered.
Middlewares are not collected either: they are listed in order under
`scribus.Middlewares`. The tag in full, with the others, is
[di/tags](../di/tags.md#quick-reference).

## Logging

In single mode the command logs the subject of every message it dispatches, at
trace level, under the message's own correlation id and trace:

```
TRACE order.created
```

The body is not in that line: Trace is a per-service switch rather than a
per-line one, so turning it on to follow dispatch would otherwise put every
payload in the log sink, and what a message carries is the service's to decide
about rather than this tool's.

The AMQP driver's own log middleware is a separate decision and currently does
include the body, one line per message, so a service that enables Trace does see
payloads today. Drop `interceptor.NewLog` from the middleware registrations if
that is not acceptable for what your queues carry.

## Next

| | |
|---|---|
| Parsing the message body — the same tags an HTTP DTO uses | [conventions/input-parsing](../conventions/input-parsing.md) |
| `Handler`, `BatchHandler`, `Consumer`, `Subjects`, `Transport` | [reference/interfaces](../reference/interfaces.md#toolsscribus-package) |
| `CommandTag` | [di/tags](../di/tags.md#scribuscommandtag) |
| Publishing an event that must not be lost | [tools/outbox](outbox.md) |
| What has to be running and in `.env` | [reference/running-locally](../reference/running-locally.md#by-module) |
| Delivering an event through the real consumer in a test | [conventions/component-harness](https://github.com/specrig/specrig-go/blob/main/waffle/docs/harness.md#delivering-an-event) |
