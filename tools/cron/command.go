package cron

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/pborman/getopt/v2"

	"github.com/ribora/failure"
	"github.com/ribora/pry"
	"github.com/ribora/session"
	"github.com/ribora/waffle/app"
)

type CronCommand interface {
	app.Command
}

type entry struct {
	expr string
	argv []string
}

// First five whitespace-separated tokens are the cron expression, the rest is argv.
var entryRe = regexp.MustCompile(`^((?:\S+\s+){4}\S+)\s+(.+)$`)

func NewCronCommand(command app.BaseCommand, schedule string, timezone string, logger pry.Logger) (*cronCommand, error) {
	tz, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}
	entries, err := parseSchedule(schedule)
	if err != nil {
		return nil, err
	}
	return &cronCommand{
		BaseCommand: command,
		timezone:    tz,
		logger:      logger,
		entries:     entries,
		closer:      make(chan struct{}, 1),
	}, nil
}

type cronCommand struct {
	app.BaseCommand
	timezone *time.Location
	logger   pry.Logger
	entries  []entry
	commands app.Commands
	closer   chan struct{}
}

func (c *cronCommand) SetCommands(cmds app.Commands) {
	c.commands = cmds
}

func (c *cronCommand) Usage(ctx context.Context) error {
	opts := getopt.New()
	var help bool
	opts.FlagLong(&help, "help", 'h', "Display this help message")
	if err := opts.Getopt(app.Args(ctx), nil); err != nil {
		return err
	}
	if help {
		opts.SetParameters("")
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", c.Description())
		opts.PrintUsage(os.Stderr)
		items := make(map[string]string, len(c.entries))
		for _, e := range c.entries {
			items[strings.Join(e.argv, " ")] = e.expr
		}
		app.PrintList(os.Stderr, "Scheduled:", items)
		return app.ErrUsage
	}
	return nil
}

func (c *cronCommand) Run(ctx context.Context) error {
	if err := c.validate(); err != nil {
		return err
	}
	if err := c.Usage(ctx); err != nil {
		return err
	}
	if len(c.entries) == 0 {
		c.logger.Trace("cron: empty schedule, nothing to run")
		return nil
	}
	scheduler, err := gocron.NewScheduler(
		gocron.WithLocation(c.timezone),
		gocron.WithStopTimeout(30*time.Minute),
	)
	if err != nil {
		return err
	}
	for _, e := range c.entries {
		if err := c.registerJob(scheduler, e); err != nil {
			return err
		}
	}
	scheduler.Start()
	<-c.closer
	return scheduler.Shutdown()
}

func (c *cronCommand) validate() error {
	if c.commands == nil {
		return failure.Wrap("cron: peer commands not injected (app.Init must run first)")
	}
	for _, e := range c.entries {
		if len(e.argv) == 0 {
			return failure.Wrap("cron: empty argv in schedule")
		}
		name := e.argv[0]
		if name == c.Name() {
			return failure.Wrap("cron: cannot schedule itself: {name}", name)
		}
		cmd := c.commands.Get(name)
		if cmd == nil {
			return failure.Wrap("cron: unknown command in schedule: {name}", name)
		}
		if _, daemon := cmd.(app.Daemon); daemon {
			return failure.Wrap("cron: command is a daemon, cannot schedule: {name}", name)
		}
	}
	return nil
}

func (c *cronCommand) registerJob(s gocron.Scheduler, e entry) error {
	cmd := c.commands.Get(e.argv[0])
	name := strings.Join(e.argv, " ")
	_, err := s.NewJob(
		gocron.CronJob(e.expr, false),
		gocron.NewTask(func(jobCtx context.Context) {
			sub := session.NewContextWithSession(jobCtx, session.Session{
				Method: "CRON",
				Uri:    cmd.Name(),
				Body:   name,
			})
			sub = app.WithArgs(sub, e.argv)
			if err := cmd.Run(sub); err != nil {
				c.logger.Error(fmt.Sprintf("cron task failed: %s", name), pry.Err(err), pry.Ctx(sub))
			}
		}),
		gocron.WithName(name),
		gocron.WithSingletonMode(gocron.LimitModeWait),
	)
	return err
}

func (c *cronCommand) Close() error {
	c.closer <- struct{}{}
	return nil
}

func parseSchedule(raw string) ([]entry, error) {
	var out []entry
	for _, line := range strings.Split(raw, "\n") {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		m := entryRe.FindStringSubmatch(l)
		if m == nil {
			return nil, failure.Wrap("invalid schedule entry: {line}", l)
		}
		out = append(out, entry{
			expr: strings.TrimSpace(m[1]),
			argv: strings.Fields(m[2]),
		})
	}
	return out, nil
}
