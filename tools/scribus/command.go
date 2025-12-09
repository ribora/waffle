package scribus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pborman/getopt/v2"

	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/waffle/app"
)

type ScribusCommand interface {
	app.Command
}

type Consumer interface {
	SetHandler(handler func(ctx context.Context, keys []string, documents [][]byte) error) error
	SetDsn(dsn string) error
	SetExchange(exchange string) error
	SetQueue(queue string) error
	SetRoutingKey(routingKey string) error
	SetPageSize(pageSize uint16) error
	SetFlushInterval(flushInterval time.Duration) error
	SetReconnectInterval(interval time.Duration) error
	Consume(ctx context.Context) error
}

type SingleHandler interface {
	Run(ctx context.Context, payload []byte) error
	Keys() []string
}

type BatchHandler interface {
	Run(ctx context.Context, keys []string, payload [][]byte) error
}

func NewScribusCommand(
	commands []SingleHandler,
	command app.BaseCommand,
	consumer Consumer,
	logger pry.Logger,
) (*scribusCommand, error) {
	if len(commands) == 0 {
		return nil, errors.New("handlers list is empty")
	}
	cmd := make(map[string]SingleHandler)
	for _, c := range commands {
		for _, key := range c.Keys() {
			cmd[key] = c
		}
	}
	err := consumer.SetHandler(func(ctx context.Context, keys []string, documents [][]byte) error {
		for idx, key := range keys {
			if handler, found := cmd[key]; found {
				err := handler.Run(ctx, documents[idx])
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
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
	command app.BaseCommand,
	consumer Consumer,
	logger pry.Logger,
) (*scribusCommand, error) {
	if err := consumer.SetHandler(handler.Run); err != nil {
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
	commands map[string]SingleHandler
	logger   pry.Logger
}

func (c *scribusCommand) Run(ctx context.Context) error {
	if err := c.Usage(); err != nil {
		return err
	}

	c.logger.Trace("The consumer has started")
	return c.consumer.Consume(ctx)
}

func (c *scribusCommand) Usage() error {
	var help bool
	getopt.FlagLong(&help, "help", 'h', "Display this help message")
	timeout := getopt.DurationLong("flush-timeout", 't', 0, "Flush every [n] seconds", "n")
	reconnect := getopt.DurationLong("reconnect-timeout", 'r', 0, "Reconnect every [n] seconds", "n")
	page := getopt.Uint16Long("page", 'p', 0, "Number of items for bulk processing")
	getopt.Parse()
	if help {
		getopt.SetParameters("[amqp://user:pass@host:port exchange query:routing.key]")
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", c.Description())
		getopt.PrintUsage(os.Stderr)
		os.Exit(2)
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
	if len(getopt.Args()) > 0 {
		if len(getopt.Args()) != 4 {
			return errors.New("unexpected connection parameters count")
		}
		_ = c.consumer.SetDsn(getopt.Args()[1])
		_ = c.consumer.SetExchange(getopt.Args()[2])
		queue := strings.Split(getopt.Args()[3], ":")
		queue = append(queue, "#")
		_ = c.consumer.SetQueue(queue[0])
		_ = c.consumer.SetRoutingKey(queue[1])
	}
	return nil
}
