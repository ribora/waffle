package cmdcommand

import (
	"context"
	"embed"
	"fmt"
	"os/exec"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("make:domain:command", "Add a CLI command (app.Command) scoped to a domain", templates, nil).
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
	projectName, err := c.FindProjectName()
	if err != nil {
		return err
	}

	if err := Generate(domain, name, projectName); err != nil {
		return err
	}

	info, err := exec.Command("make", ".tidy", "generate").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	return nil
}
