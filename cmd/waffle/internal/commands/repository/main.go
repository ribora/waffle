package repository

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

func New() *cmd {
	return &cmd{command.New("make:domain:repository", "Add a repository to an existing domain (only sql is supported)", templates, nil).
		WithUsage("<type> <domain> <entity>")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	if err := command.RejectFlags(args); err != nil {
		return fmt.Errorf("%w\n%s", err, c.Usage())
	}
	if len(args) < 3 {
		return errors.New(c.Usage())
	}
	if err := command.ValidateName("domain", args[1]); err != nil {
		return err
	}
	if err := command.ValidateName("entity", args[2]); err != nil {
		return err
	}
	storage := args[0]
	if storage != "sql" {
		return fmt.Errorf("storage type %q is not supported (only sql for now)", storage)
	}
	domain := args[1]
	entity := args[2]
	projectName, err := c.FindProjectName()
	if err != nil {
		return err
	}

	if err := GenerateSecondary(domain, entity, projectName); err != nil {
		return err
	}

	info, err := exec.Command("make", ".tidy", "generate").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	return nil
}
