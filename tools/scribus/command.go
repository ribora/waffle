package scribus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/pborman/getopt/v2"

	"github.com/ribora/pry"
	"github.com/ribora/waffle/app"
	"github.com/ribora/whisk/scan"
)

type commandTag struct{}

var CommandTag = commandTag{}

type ScribusCommand interface {
	app.Command
}

type Subjects interface {
	Of(src scan.Source) string
	Match(pattern, subject string) bool
	Overlap(a, b string) bool
}

type Transport interface {
	Params() []string
	Apply(args []string) error
}

type Consumer interface {
	Transport

	SetHandler(handler func(msgs []scan.Source) error) error
	Bind(subject string) error
	SetPageSize(pageSize uint16) error
	SetFlushInterval(flushInterval time.Duration) error
	SetReconnectInterval(interval time.Duration) error
	Consume(ctx context.Context) error
}

type Middleware interface {
	Wrap(next func(msgs []scan.Source) error) func(msgs []scan.Source) error
}

type Middlewares []Middleware

func NewMiddlewares(middlewares []Middleware) Middlewares { return middlewares }

func applyMiddlewares(handler func(msgs []scan.Source) error, middlewares Middlewares) func(msgs []scan.Source) error {
	for _, s := range slices.Backward(middlewares) {
		handler = s.Wrap(handler)
	}
	return handler
}

// The broker routes by the bindings before a real consumer sees anything, so only
// a consumer standing in for the broker asks for the driver's rule.
func route(consumer Consumer, subjects Subjects) error {
	if router, ok := consumer.(interface{ SetSubjects(Subjects) error }); ok {
		return router.SetSubjects(subjects)
	}
	return nil
}

type Handler interface {
	Run(src scan.Source) error
	Keys() []string
}

type BatchHandler interface {
	Run(msgs []scan.Source) error
	Keys() []string
}

func NewScribusCommand(
	commands []Handler,
	middlewares Middlewares,
	subjects Subjects,
	command app.BaseCommand,
	consumer Consumer,
	logger pry.Logger,
) (*scribusCommand, error) {
	if len(commands) == 0 {
		return nil, errors.New("handlers list is empty")
	}
	type bound struct {
		key string
		h   Handler
	}
	var bounds []bound
	for _, c := range commands {
		for _, key := range c.Keys() {
			for _, prev := range bounds {
				if subjects.Overlap(prev.key, key) {
					return nil, fmt.Errorf("subjects overlap: %q and %q — give them separate consumers", prev.key, key)
				}
			}
			bounds = append(bounds, bound{key: key, h: c})
			if err := consumer.Bind(key); err != nil {
				return nil, err
			}
		}
	}
	// No map in front of Match, not even for a subject with no wildcard: what counts
	// as equal is the driver's answer, and a map would pass every test here anyway.
	handler := func(msgs []scan.Source) error {
		for _, m := range msgs {
			subject := subjects.Of(m)
			var h Handler
			for _, b := range bounds {
				if subjects.Match(b.key, subject) {
					h = b.h
					break
				}
			}
			if h == nil {
				continue
			}
			// The subject, not the body: what a message carries is the service's to
			// decide about, and Trace is a per-service toggle, not a per-line one.
			logger.Trace(subject, pry.Ctx(m.Context()))
			if err := h.Run(m); err != nil {
				return err
			}
		}
		return nil
	}
	if err := consumer.SetHandler(applyMiddlewares(handler, middlewares)); err != nil {
		return nil, err
	}
	if err := route(consumer, subjects); err != nil {
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
	middlewares Middlewares,
	subjects Subjects,
	command app.BaseCommand,
	consumer Consumer,
	logger pry.Logger,
) (*scribusCommand, error) {
	keys := handler.Keys()
	if len(keys) == 0 {
		return nil, errors.New("batch handler exposes no subjects")
	}
	for _, key := range keys {
		if err := consumer.Bind(key); err != nil {
			return nil, err
		}
	}
	if err := consumer.SetHandler(applyMiddlewares(handler.Run, middlewares)); err != nil {
		return nil, err
	}
	if err := route(consumer, subjects); err != nil {
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
	// Set by the constructor, so Usage can refuse a --page the mode cannot honour.
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
	timeout := opts.DurationLong("flush-timeout", 't', 0, "Flush every [n], a duration such as 5s", "n")
	reconnect := opts.DurationLong("reconnect-timeout", 'r', 0, "Reconnect every [n], a duration such as 5m", "n")
	page := opts.Uint16Long("page", 'p', 0, "Number of items for bulk processing")
	if err := opts.Getopt(app.Args(ctx), nil); err != nil {
		return err
	}
	params := c.consumer.Params()
	if help {
		opts.SetParameters("[" + strings.Join(params, " ") + "]")
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
	// A page settles as one unit, so in single mode one failure requeues the
	// neighbours of the message that failed. Refused rather than documented.
	if *page > 1 && !c.batch {
		return fmt.Errorf("--page %d needs a BatchHandler: single mode calls the handler once per message, "+
			"but %d of them settle together, so one failure requeues the others", *page, *page)
	}
	// Only the explicit 1: an absent flag leaves the size to the container, which
	// this function cannot read back.
	if *page == 1 && c.batch {
		c.logger.Warn("--page 1 hands the batch handler one message at a time")
	}
	if *page != 0 {
		_ = c.consumer.SetPageSize(*page)
	}
	if args := opts.Args(); len(args) > 0 {
		if len(args) != len(params) {
			return fmt.Errorf("expected %d connection parameters (%s), got %d",
				len(params), strings.Join(params, " "), len(args))
		}
		if err := c.consumer.Apply(args); err != nil {
			return err
		}
	}
	return nil
}

func (c *scribusCommand) Daemon() {}
