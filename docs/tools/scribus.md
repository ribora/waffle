# Scribus Tool

The scribus tool wraps `gitlab.com/gorib/amqp` into a waffle `Command` for consuming RabbitMQ messages. It supports two modes: **single mode**, where each routing key is handled by a dedicated `Handler`, and **batch mode**, where a single `BatchHandler` processes all messages together.

## Setup

Scribus ships **two** mode-specific subpackages — pick one. Common registrations (BaseCommand, AMQP Consumer) live in `tools/scribus` and are pulled in transitively.

**Single mode** (each routing key dispatched to a dedicated `Handler`):

```go
import "gitlab.com/gorib/di"

func InitDi() {
    di.Include("gitlab.com/gorib/waffle/config/tools/scribus/single")
    // ... other registrations ...
}
```

This expands to the registrations from [`tools/scribus/init.go`](../../tools/scribus/init.go) (common) plus [`tools/scribus/single/init.go`](../../tools/scribus/single/init.go):

```go
// common — from tools/scribus
di.Wire[app.BaseCommand](
    func() app.BaseCommand { return app.NewCommand("scribus", "Start RabbitMq consumer") },
    di.For[wscribus.ScribusCommand](),
)
di.Wire[wscribus.Consumer](amqp.NewConsumer, di.Args(map[string]any{
    "dsn":        env.NeedValue[string]("RABBIT_DSN"),
    "exchange":   env.NeedValue[string]("RABBIT_EXCHANGE"),
    "queue":      env.NeedValue[string]("RABBIT_QUEUE"),
    "routingKey": "#",
    "opts": []amqp.ConsumerOptions{
        amqp.WithConsumerPageSize(1),
        amqp.WithConsumerFlushInterval(5 * time.Second),
    },
}))

// single-mode — from tools/scribus/single
di.Wire[wscribus.ScribusCommand](wscribus.NewScribusCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
    "commands": di.Tags[wscribus.Handler](wscribus.CommandTag),
}))
```

**Batch mode** (one `BatchHandler` processes all messages):

```go
func InitDi() {
    di.Include("gitlab.com/gorib/waffle/config/tools/scribus/batch")
    // ... and register your own BatchHandler implementation:
    di.Wire[wscribus.BatchHandler](scribusCommand.NewNotificationAcceptedCommand)
}
```

`tools/scribus/batch` pulls in the same common block plus `di.Wire[ScribusCommand](NewBatchScribusCommand, ...)`. Your `BatchHandler` registration stays in your own `InitDi()`.

Run `make generate` after adding this.

## Running the consumer

```sh
./app scribus <dsn> <exchange> <queue>:<routing-key>
```

Example:

```sh
./app scribus amqp://guest:guest@localhost:5672 events orders:order.created
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--flush-timeout` | `-t` | `0` (use default) | Flush every [n] seconds |
| `--page` | `-p` | `0` (use default) | Number of items for bulk processing |
| `--reconnect-timeout` | `-r` | `0` (use default) | Reconnect every [n] seconds |

## Environment variables

Environment variables are not read automatically. They are wired explicitly in `InitDi()` via `env.NeedValue` / `env.Value` as shown in the Setup section above. Variable names are chosen by you. The CLI positional arguments take precedence at runtime.

## Single mode

In single mode, each routing key dispatches to its own `Handler`.

### Handler interface

```go
type Handler interface {
    Keys() []string
    Run(ctx context.Context, payload []byte) error
}
```

`Keys()` returns the routing keys this handler subscribes to. `Run` is called for each incoming message.

### Message model

Define a local struct for the message payload in `cmd/scribus/model/`. It is separate from the domain model and can have `json` tags:

```go
// cmd/scribus/model/order.go
package model

type OrderCreated struct {
    Id         int            `json:"id"`
    CustomerId int            `json:"customerId"`
    Items      []OrderItem    `json:"items"`
}
```

### Implementing a handler

The handler defines its own management interface (consumer-defined, same pattern as gRPC). The struct is private; the constructor is public.

```go
// cmd/scribus/order_created.go
package scribus

import (
    "context"

    scribusModel "mymodule/domain/order/cmd/scribus/model"
    "mymodule/domain/order/model"
    "mymodule/pkg/validator"
    "gitlab.com/gorib/pry"
)

type OrderManagement interface {
    HandleOrderCreated(ctx context.Context, order *model.Order) error
}

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

func (h *orderCreatedHandler) Run(ctx context.Context, payload []byte) error {
    msg, err := validator.Parse[scribusModel.OrderCreated](payload)
    if err != nil {
        h.logger.Error("Cannot parse message", pry.Err(err), pry.Ctx(ctx))
        return nil // ACK: malformed message, re-queuing won't help
    }

    err = h.management.HandleOrderCreated(ctx, &model.Order{
        Id:         msg.Id,
        CustomerId: msg.CustomerId,
    })
    if err != nil {
        h.logger.Error("Cannot handle order.created", pry.Err(err), pry.Ctx(ctx))
    }
    return nil // ACK: log the error and move on
}
```

**Error handling**: scribus handlers always return `nil`. If processing fails, log the error and acknowledge the message. Returning a non-nil error causes the message to be re-queued, which is rarely the right behaviour for event-driven notifications.

### Registering a handler

Tag with `wscribus.CommandTag`. Wire to the `scribus.Handler` interface:

```go
di.Wire[wscribus.Handler](scribus.NewOrderCreatedHandler, di.Tag(wscribus.CommandTag))
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
    Run(ctx context.Context, keys []string, payloads [][]byte) error
}
```

`keys[i]` is the routing key of `payloads[i]`. In practice all messages in a batch usually share the same routing key.

### Implementing a batch handler

```go
type orderBatchHandler struct {
    management OrderManagement
    logger     pry.Logger
}

func NewOrderBatchHandler(management OrderManagement, logger pry.Logger) *orderBatchHandler {
    return &orderBatchHandler{management: management, logger: logger}
}

func (h *orderBatchHandler) Run(ctx context.Context, keys []string, payloads [][]byte) error {
    for i, payload := range payloads {
        msg, err := validator.Parse[scribusModel.OrderCreated](payload)
        if err != nil {
            h.logger.Error("Cannot parse message", pry.Err(err), pry.Ctx(ctx))
            continue
        }
        if err = h.management.HandleOrderCreated(ctx, &model.Order{Id: msg.Id}); err != nil {
            h.logger.Error("Cannot handle "+keys[i], pry.Err(err), pry.Ctx(ctx))
        }
    }
    return nil
}
```

Register exactly one `BatchHandler` without a tag:

```go
di.Wire[wscribus.BatchHandler](scribus.NewOrderBatchHandler)
```

## How routing works

On startup the scribus command:

1. Parses `<queue>:<routing-key>` from the positional argument (if provided)
2. For each incoming message, reads the routing key from the AMQP delivery
3. In single mode: looks up the handler by key and calls `Run`
4. In batch mode: accumulates messages until `--page` is reached or `--flush-timeout` expires, then calls `BatchHandler.Run` with the batch

## DI tags

| Tag | Collects |
|-----|---------|
| `scribus.CommandTag` | All `Handler` implementations (single mode) |

`BatchHandler` is provided without a tag — only one may be registered.

## Logging

The scribus command logs every consumed message at trace level in single mode:

```
TRACE order.created {"id":42,...}
```
