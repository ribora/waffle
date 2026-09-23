package agents

import (
	"context"
	"embed"
	"slices"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

var noTemplates embed.FS

func New() *cmd {
	return &cmd{
		base: command.New("agents",
			"Regenerate AGENTS.md with project conventions and auto-detected service context",
			noTemplates, nil).
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
	quiet := false
	if i := slices.Index(args, "-q"); i >= 0 {
		quiet = true
		args = slices.Delete(args, i, i+1)
	}
	if err := c.base.Operands(args, 0, 0); err != nil {
		return err
	}
	projectName, err := c.base.FindProjectName()
	if err != nil {
		return err
	}
	_, err = Generate(projectName, quiet)
	return err
}
