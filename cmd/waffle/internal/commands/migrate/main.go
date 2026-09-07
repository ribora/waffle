package migrate

import (
	"context"
	"embed"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("make:domain:migrate", "Set up migrations to the specified domain", templates, nil).
		WithUsage("<domain>")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	if err := c.Operands(args, 1, 1); err != nil {
		return err
	}
	if err := command.ValidateName("domain", args[0]); err != nil {
		return err
	}
	domain := args[0]
	projectName, err := c.FindProjectName()
	if err != nil {
		return err
	}
	return Generate(domain, projectName)
}
