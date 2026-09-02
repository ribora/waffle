package scribus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/pborman/getopt/v2"

	"gitlab.com/gorib/amqp"
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
	SetHandler(handler func(msgs []amqp.Source) error) error
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
	Wrap(next func(msgs []amqp.Source) error) func(msgs []amqp.Source) error
}

func applyMiddlewares(handler func(msgs []amqp.Source) error, middlewares []Middleware) func(msgs []amqp.Source) error {
	sorted := append([]Middleware(nil), middlewares...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Order() < sorted[j].Order() })
	for i := len(sorted) - 1; i >= 0; i-- {
		handler = sorted[i].Wrap(handler)
	}
	return handler
}

type Handler interface {
	Run(src amqp.Source) error
	Keys() []string
}

type BatchHandler interface {
	Run(msgs []amqp.Source) error
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
	type wildcard struct {
		pat string
		h   Handler
	}
	var (
		exact = make(map[string]Handler)
		wild  []wildcard
		keys  []string
	)
	for _, c := range commands {
		for _, key := range c.Keys() {
			for _, prev := range keys {
				if overlap(prev, key) {
					return nil, fmt.Errorf("routing keys overlap: %q and %q — give them separate consumers", prev, key)
				}
			}
			keys = append(keys, key)
			if strings.ContainsAny(key, "*#") {
				wild = append(wild, wildcard{pat: key, h: c})
			} else {
				exact[key] = c
			}
			if err := consumer.Bind(key); err != nil {
				return nil, err
			}
		}
	}
	handler := func(msgs []amqp.Source) error {
		for _, m := range msgs {
			rk := m.RoutingKey()
			h, found := exact[rk]
			if !found {
				for _, w := range wild {
					if matchTopic(w.pat, rk) {
						h, found = w.h, true
						break
					}
				}
			}
			if !found {
				continue
			}
			logger.Trace(fmt.Sprintf("%s %s", rk, string(m.Body())))
			if err := h.Run(m); err != nil {
				return err
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
		batch:       true,
	}, nil
}

type scribusCommand struct {
	app.BaseCommand
	consumer Consumer
	logger   pry.Logger
	// Which constructor built it. --page settles messages in groups, and only a
	// BatchHandler is written to expect that, so the flag has to agree.
	batch bool
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
	// A page settles as one unit: Ack/Nack carry `multiple` once it holds more
	// than one message. A single-mode Handler is called per message and reads as
	// if it settled per message too, so the combination requeues the neighbours
	// of whatever failed and skips the rest of the page. Refused rather than
	// documented.
	if *page > 1 && !c.batch {
		return fmt.Errorf("--page %d needs a BatchHandler: single mode calls the handler once per message, "+
			"but %d of them settle together, so one failure requeues the others", *page, *page)
	}
	// Only the explicit 1: an absent flag leaves the size to the container, and
	// this function cannot read it back to know whether it is worth mentioning.
	if *page == 1 && c.batch {
		c.logger.Warn("--page 1 hands the batch handler one message at a time")
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
