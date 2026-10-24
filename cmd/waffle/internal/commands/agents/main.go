package agents

import (
	"context"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

func New() *cmd {
	return &cmd{
		base: command.New("agents",
			"Regenerate waffle.md with the framework's conventions and auto-detected service context").
			WithUsage("[-q]\n" +
				"  -q reports only what changed; make generate runs it on every build"),
	}
}

type cmd struct {
	base *command.Command
}

func (c *cmd) Code() string        { return c.base.Code() }
func (c *cmd) Description() string { return c.base.Description() }
func (c *cmd) Usage() string       { return c.base.Usage() }

func (c *cmd) Run(ctx context.Context, args ...string) error {
	quiet, args := command.TakeBool(args, "-q")
	if err := c.base.Operands(args, 0, 0); err != nil {
		return err
	}
	p, err := command.OpenProject()
	if err != nil {
		return err
	}
	_, err = Generate(p, quiet)
	return err
}
