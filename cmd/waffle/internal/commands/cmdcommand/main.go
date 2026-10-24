package cmdcommand

import (
	"context"
	"embed"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templateFiles embed.FS

var templates = command.NewTemplates(templateFiles)

func New() *cmd {
	return &cmd{command.New("make:domain:command", "Add a CLI command (app.Command) scoped to a domain").
		WithUsage("<domain> <command>")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	if err := c.Operands(args, 2, 2); err != nil {
		return err
	}
	domain := args[0]
	if err := command.ValidateName("domain", domain); err != nil {
		return err
	}
	// Not ValidateName: a CLI token may have hyphens.
	name, err := command.ParseCommandName("command", args[1])
	if err != nil {
		return err
	}
	p, err := command.OpenProject()
	if err != nil {
		return err
	}
	if err := p.RequireDomain(domain); err != nil {
		return err
	}

	if err := Generate(p, domain, name); err != nil {
		return err
	}

	_, err = p.Run("make", ".tidy", "generate")
	return err
}
