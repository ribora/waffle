package scribus

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pborman/getopt/v2"

	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/server/rabbit"
	"gitlab.com/gorib/waffle/app"
)

type ScribusCommand interface {
	app.Command
}

type BusConsumer interface {
	Start(connection rabbit.Topic, commands map[string]rabbit.Handler) error
}

func NewScribusCommand(
	commands []rabbit.Handler,
	command app.BaseCommand,
	consumer BusConsumer,
	connection rabbit.Topic,
	logger pry.Logger,
) *scribusCommand {
	cmd := make(map[string]rabbit.Handler)
	for _, command := range commands {
		for _, key := range command.Keys() {
			cmd[key] = command
		}
	}
	return &scribusCommand{
		commands:    cmd,
		consumer:    consumer,
		stop:        make(chan struct{}, 1),
		connection:  connection,
		BaseCommand: command,
		logger:      logger,
	}
}

type scribusCommand struct {
	app.BaseCommand
	consumer   BusConsumer
	connection rabbit.Topic
	stop       chan struct{}
	commands   map[string]rabbit.Handler
	logger     pry.Logger
}

func (c *scribusCommand) Run(context.Context) error {
	if err := c.Usage(); err != nil {
		return err
	}

	c.logger.Trace("The consumer has started")

	for {
		if err := c.consumer.Start(c.connection, c.commands); err != nil {
			return err
		}
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-c.stop:
			return nil
		case <-timer.C:
		}
	}
}

func (c *scribusCommand) Close() error {
	c.stop <- struct{}{}
	return nil
}

func (c *scribusCommand) Usage() error {
	var help bool
	getopt.FlagLong(&help, "help", 'h', "Display this help message")
	timeout := getopt.IntLong("timeout", 't', 0, "Reconnect to RabbitMQ every [n] seconds", "n")
	getopt.Parse()
	if help {
		getopt.SetParameters("[amqp://user:pass@host:port exchange query:routing.key]")
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", c.Description())
		getopt.PrintUsage(os.Stderr)
		os.Exit(2)
	}

	if *timeout != 0 {
		c.consumer = rabbit.NewBusConsumer(time.Duration(*timeout)*time.Second, c.logger)
	}
	if len(getopt.Args()) > 3 {
		return fmt.Errorf("unexpected connection parameters count")
	}
	if len(getopt.Args()) > 0 {
		queue := strings.Split(getopt.Args()[2], ":")
		c.connection = rabbit.NewTopic(getopt.Args()[0], getopt.Args()[1], queue[0], queue[1])
	}
	return nil
}
