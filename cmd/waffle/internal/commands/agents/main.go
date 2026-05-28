package agents

import (
	"context"
	"embed"
	"fmt"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

var noTemplates embed.FS

func New() *cmd {
	return &cmd{
		base: command.New("agents",
			"Regenerate AGENTS.md with project conventions and auto-detected service context",
			noTemplates, nil),
	}
}

type cmd struct {
	base *command.Command
}

func (c *cmd) Code() string        { return c.base.Code() }
func (c *cmd) Description() string { return c.base.Description() }

func (c *cmd) Run(ctx context.Context, args ...string) error {
	projectName, err := c.base.FindProjectName()
	if err != nil {
		return err
	}
	if err := Generate(projectName); err != nil {
		return err
	}
	fmt.Println("Wrote AGENTS.md (+ symlinks CLAUDE.md, .cursorrules).")
	fmt.Println("Service-specific notes go in CONTRIBUTING.md.")
	return nil
}
