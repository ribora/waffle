# Scribus Tool

The scribus tool wraps `gitlab.com/gorib/amqp` into a waffle `Command` for consuming RabbitMQ messages. It supports two modes: **single mode**, where each routing key is handled by a dedicated `Handler`, and **batch mode**, where a single `BatchHandler` processes all messages together.

## Setup

Copy the scribus registrations into `InitDi()` in `config/di/di.go`.

**Single mode** (each routing key dispatched to a dedicated `Handler`):

```go
import (
    "time"
    "gitlab.com/gorib/amqp"
    "gitlab.com/gorib/di"
    "gitlab.com/gorib/env"
    "gitlab.com/gorib/waffle/app"
    wscribus "gitlab.com/gorib/waffle/tools/scribus"
)

// Inside InitDi():
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
di.Wire[wscribus.ScribusCommand](wscribus.NewScribusCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
    "commands": di.Tags[wscribus.Handler](wscribus.CommandTag),
}))
```

**Batch mode** (one `BatchHandler` processes all messages):

```go
// Replace the NewScribusCommand registration with:
di.Wire[wscribus.BatchHandler](scribusCommand.NewNotificationAcceptedCommand)
di.Wire[wscribus.ScribusCommand](wscribus.NewBatchScribusCommand, di.Tag(app.CommandTag))
```

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

### Implementing a handler

```go
type OrderCreatedHandler struct {
    service *order.Service
}

func NewOrderCreatedHandler(s *order.Service) *OrderCreatedHandler {
    return &OrderCreatedHandler{service: s}
}

func (h *OrderCreatedHandler) Keys() []string {
    return []string{"order.created"}
}

func (h *OrderCreatedHandler) Run(ctx context.Context, payload []byte) error {
    var event OrderCreatedEvent
    if err := json.Unmarshal(payload, &event); err != nil {
        return err
    }
    return h.service.HandleOrderCreated(ctx, event)
}
```

### Registering a handler

Tag with `scribus.CommandTag`:

```go
di.Wire[scribus.Handler](NewOrderCreatedHandler, di.Tag(wscribus.CommandTag))
```

All tagged handlers are collected and dispatched to by routing key.

## Batch mode

In batch mode, a single `BatchHandler` receives all messages in a slice, grouped by routing key.

### BatchHandler interface

```go
type BatchHandler interface {
    Run(ctx context.Context, keys []string, payloads [][]byte) error
}
```

`keys[i]` is the routing key of `payloads[i]`.

### Implementing a batch handler

```go
type EventBatchHandler struct {
    service *analytics.Service
}

func (h *EventBatchHandler) Run(ctx context.Context, keys []string, payloads [][]byte) error {
    events := make([]analytics.Event, len(payloads))
    for i, p := range payloads {
        events[i] = analytics.Event{Key: keys[i], Payload: p}
    }
    return h.service.BulkIngest(ctx, events)
}
```

Register exactly one `BatchHandler` without a tag:

```go
di.Wire[scribus.BatchHandler](NewEventBatchHandler)
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
