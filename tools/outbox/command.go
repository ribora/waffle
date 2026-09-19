package outbox

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pborman/getopt/v2"

	"gitlab.com/gorib/failure"
	"gitlab.com/gorib/waffle/app"
)

type OutboxCommand interface {
	app.Command
}

func NewOutboxCommand(command app.BaseCommand, relay Relay, store Store) *outboxCommand {
	return &outboxCommand{BaseCommand: command, relay: relay, store: store}
}

type outboxCommand struct {
	app.BaseCommand
	relay Relay
	store Store
}

func (c *outboxCommand) parse(ctx context.Context) ([]string, error) {
	opts := getopt.New()
	var help bool
	opts.FlagLong(&help, "help", 'h', "Display this help message")
	if err := opts.Getopt(app.Args(ctx), nil); err != nil {
		return nil, err
	}
	if help {
		opts.SetParameters("[command]")
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", c.Description())
		opts.PrintUsage(os.Stderr)
		app.PrintList(os.Stderr, "Commands:", map[string]string{
			"run":        "Drain the outbox until stopped (the default)",
			"status":     "Print the backlog and the age of the oldest unsent entry",
			"reset [id]": "Clear the attempts of entries that ran out, so they are sent again; an id takes that one whatever its attempts",
		})
		return nil, app.ErrUsage
	}
	return opts.Args(), nil
}

func (c *outboxCommand) Usage(ctx context.Context) error {
	_, err := c.parse(ctx)
	return err
}

func (c *outboxCommand) Run(ctx context.Context) error {
	args, err := c.parse(ctx)
	if err != nil {
		return err
	}
	command := "run"
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}
	switch command {
	case "run":
		return c.relay.Run(ctx)
	case "status":
		return c.status(ctx)
	case "reset":
		return c.reset(ctx, args)
	default:
		return failure.Wrap("outbox: unknown command {name}", command)
	}
}

func (c *outboxCommand) status(ctx context.Context) error {
	backlog, err := c.store.Backlog(ctx)
	if err != nil {
		return err
	}
	oldest := "-"
	if backlog.Unsent > 0 && !backlog.OldestAt.IsZero() {
		oldest = time.Since(backlog.OldestAt).Truncate(time.Second).String()
	}
	_, err = fmt.Fprintf(os.Stdout, "unsent\t%d\nfailed\t%d\noldest\t%s\n", backlog.Unsent, backlog.Failed, oldest)
	return err
}

func (c *outboxCommand) reset(ctx context.Context, args []string) error {
	var id string
	if len(args) > 0 {
		id = args[0]
	}
	reset, err := c.store.Reset(ctx, id)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "reset\t%d\n", reset)
	return err
}

func (c *outboxCommand) Daemon() {}

func (c *outboxCommand) Close() error {
	return c.relay.Close()
}
