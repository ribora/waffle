package scribus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/pborman/getopt/v2"

	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/waffle/app"
)

type (
	commandTag    struct{}
	middlewareTag struct{}
)

var (
	CommandTag    = commandTag{}
	MiddlewareTag = middlewareTag{}
)

type ScribusCommand interface {
	app.Command
}

type Consumer interface {
	SetHandler(handler func(ctx context.Context, keys []string, documents [][]byte) error) error
	SetDsn(dsn string) error
	SetExchange(exchange string) error
	SetQueue(queue string) error
	Bind(routingKey string) error
	SetPageSize(pageSize uint16) error
	SetFlushInterval(flushInterval time.Duration) error
	SetReconnectInterval(interval time.Duration) error
	Consume(ctx context.Context) error
}

type Middleware interface {
	Name() string
	Order() int
	Wrap(next func(ctx context.Context, keys []string, documents [][]byte) error) func(ctx context.Context, keys []string, documents [][]byte) error
}

func applyMiddlewares(handler func(ctx context.Context, keys []string, documents [][]byte) error, middlewares []Middleware) func(ctx context.Context, keys []string, documents [][]byte) error {
	sorted := append([]Middleware(nil), middlewares...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Order() < sorted[j].Order() })
	for i := len(sorted) - 1; i >= 0; i-- {
		handler = sorted[i].Wrap(handler)
	}
	return handler
}

type Handler interface {
	Run(ctx context.Context, payload []byte) error
	Keys() []string
}

type BatchHandler interface {
	Run(ctx context.Context, keys []string, payload [][]byte) error
	Keys() []string
}

func NewScribusCommand(
	commands []Handler,
	middlewares []Middleware,
	command app.BaseCommand,
	consumer Consumer,
	logger pry.Logger,
) (*scribusCommand, error) {
	if len(commands) == 0 {
		return nil, errors.New("handlers list is empty")
	}
	cmd := make(map[string]Handler)
	for _, c := range commands {
		for _, key := range c.Keys() {
			cmd[key] = c
			if err := consumer.Bind(key); err != nil {
				return nil, err
			}
		}
	}
	handler := func(ctx context.Context, keys []string, documents [][]byte) error {
		for idx, key := range keys {
			if h, found := cmd[key]; found {
				logger.Trace(fmt.Sprintf("%s %s", keys[idx], string(documents[idx])))
				if err := h.Run(ctx, documents[idx]); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := consumer.SetHandler(applyMiddlewares(handler, middlewares)); err != nil {
		return nil, err
	}

	return &scribusCommand{
		consumer:    consumer,
		BaseCommand: command,
		logger:      logger,
	}, nil
}

func NewBatchScribusCommand(
	handler BatchHandler,
	middlewares []Middleware,
	command app.BaseCommand,
	consumer Consumer,
	logger pry.Logger,
) (*scribusCommand, error) {
	keys := handler.Keys()
	if len(keys) == 0 {
		return nil, errors.New("batch handler exposes no routing keys")
	}
	for _, key := range keys {
		if err := consumer.Bind(key); err != nil {
			return nil, err
		}
	}
	if err := consumer.SetHandler(applyMiddlewares(handler.Run, middlewares)); err != nil {
		return nil, err
	}

	return &scribusCommand{
		consumer:    consumer,
		BaseCommand: command,
		logger:      logger,
	}, nil
}

type scribusCommand struct {
	app.BaseCommand
	consumer Consumer
	commands map[string]Handler
	logger   pry.Logger
}

func (c *scribusCommand) Run(ctx context.Context) error {
	if err := c.Usage(ctx); err != nil {
		return err
	}

	c.logger.Trace("The consumer has started")
	return c.consumer.Consume(ctx)
}

func (c *scribusCommand) Usage(ctx context.Context) error {
	opts := getopt.New()
	var help bool
	opts.FlagLong(&help, "help", 'h', "Display this help message")
	timeout := opts.DurationLong("flush-timeout", 't', 0, "Flush every [n] seconds", "n")
	reconnect := opts.DurationLong("reconnect-timeout", 'r', 0, "Reconnect every [n] seconds", "n")
	page := opts.Uint16Long("page", 'p', 0, "Number of items for bulk processing")
	if err := opts.Getopt(app.Args(ctx), nil); err != nil {
		return err
	}
	if help {
		opts.SetParameters("[amqp://user:pass@host:port exchange queue]")
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", c.Description())
		opts.PrintUsage(os.Stderr)
		return app.ErrUsage
	}

	if *timeout != 0 {
		if err := c.consumer.SetFlushInterval(*timeout); err != nil {
			return err
		}
	}
	if *reconnect != 0 {
		if err := c.consumer.SetReconnectInterval(*reconnect); err != nil {
			return err
		}
	}
	if *page != 0 {
		_ = c.consumer.SetPageSize(*page)
	}
	if len(opts.Args()) > 0 {
		if len(opts.Args()) != 4 {
			return errors.New("unexpected connection parameters count")
		}
		_ = c.consumer.SetDsn(opts.Args()[1])
		_ = c.consumer.SetExchange(opts.Args()[2])
		_ = c.consumer.SetQueue(opts.Args()[3])
	}
	return nil
}

func (c *scribusCommand) Daemon() {}
