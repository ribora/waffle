# Scribus Tool

The scribus tool wraps `gitlab.com/gorib/amqp` into a waffle `Command` for consuming RabbitMQ messages. It supports two modes: **single mode**, where each routing key is handled by a dedicated `Handler`, and **batch mode**, where a single `BatchHandler` processes all messages together.

## Setup

Copy the scribus registrations into `InitDi()` in `config/di/di.go`. The consumer arguments are passed directly as map keys — there is no `amqp.ConsumerOptions` slice.

**Batch mode** (one `BatchHandler` processes all messages):

```go
import (
    "time"
    "gitlab.com/gorib/amqp"
    "gitlab.com/gorib/di"
    "gitlab.com/gorib/env"
    "gitlab.com/gorib/waffle/app"
    wscribus "gitlab.com/gorib/waffle/tools/scribus"
    scribusCommand "mymodule/domain/notification/cmd/scribus"
)

// Inside InitDi():
di.Wire[wscribus.BatchHandler](scribusCommand.NewNotificationAcceptedCommand)
di.Wire[app.BaseCommand](
    func() app.BaseCommand { return app.NewCommand("scribus", "Start RabbitMq consumer") },
    di.For[wscribus.ScribusCommand](),
)
di.Wire[wscribus.Consumer](amqp.NewConsumer, di.Args(map[string]any{
    "dsn":           env.NeedValue[string]("RABBIT_DSN"),
    "exchange":      env.NeedValue[string]("RABBIT_EXCHANGE"),
    "queue":         env.NeedValue[string]("RABBIT_QUEUE"),
    "routingKey":    "#",
    "pageSize":      uint16(1),
    "flushInterval": time.Hour,
}))
di.Wire[wscribus.ScribusCommand](wscribus.NewBatchScribusCommand, di.Tag(app.CommandTag))
```

**Single mode** (each routing key dispatched to a dedicated `Handler`):

```go
di.Wire[app.BaseCommand](
    func() app.BaseCommand { return app.NewCommand("scribus", "Start RabbitMq consumer") },
    di.For[wscribus.ScribusCommand](),
)
di.Wire[wscribus.Consumer](amqp.NewConsumer, di.Args(map[string]any{
    "dsn":           env.NeedValue[string]("RABBIT_DSN"),
    "exchange":      env.NeedValue[string]("RABBIT_EXCHANGE"),
    "queue":         env.NeedValue[string]("RABBIT_QUEUE"),
    "routingKey":    "#",
    "pageSize":      uint16(1),
    "flushInterval": 5 * time.Second,
}))
di.Wire[wscribus.ScribusCommand](wscribus.NewScribusCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
    "commands": di.Tags[wscribus.Handler](wscribus.CommandTag),
}))
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
| `--flush-timeout` | `-t` | `5s` | How long to wait before flushing a partial batch |
| `--page` | `-p` | `1` | Batch size (number of messages per flush) |
| `--reconnect-timeout` | `-r` | — | How long to wait before reconnecting after a connection drop |

## Environment variables

When environment variables are set, the consumer uses them as defaults (the CLI flags take precedence):

| Variable | Description |
|----------|-------------|
| `RABBIT_DSN` | AMQP connection string |
| `RABBIT_EXCHANGE` | Exchange name |
| `RABBIT_QUEUE` | Queue name |
| `RABBIT_DLX_EXCHANGE` | Dead-letter exchange name (default: `"errors"`) |

See [reference/environment.md](../reference/environment.md) for the full list.

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

## How routing works

On startup the scribus command:

1. Parses `<queue>:<routing-key>` from the positional argument
2. Registers the queue with the exchange using that routing key
3. For each incoming message, reads the routing key from the AMQP delivery
4. In single mode: looks up the handler by key and calls `Run`
5. In batch mode: accumulates messages until `--page` is reached or `--flush-timeout` expires, then calls `BatchHandler.Run` with the batch

## Dead-letter exchange

Unacknowledged messages (where `Run` returns an error) are routed to the dead-letter exchange. The default DLX name is `"errors"`, configurable via `RABBIT_DLX_EXCHANGE`.

## DI tags

| Tag | Collects |
|-----|---------|
| `scribus.CommandTag` | All `Handler` implementations (single mode) |

`BatchHandler` is provided without a tag — only one may be registered.

## Logging

The scribus command logs every consumed message at trace level:

```
TRACE routing_key=order.created payload={"id":42,...}
```

Errors from `Run` or `BatchHandler.Run` are logged at error level with the full `failure.Error` context including stack trace.
