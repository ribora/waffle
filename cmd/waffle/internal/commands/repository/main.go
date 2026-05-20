package repository

import (
	"context"
	"fmt"
	"os/exec"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

func New() *cmd {
	return &cmd{command.New("make:domain:repository", "Add a repository to an existing domain (only sql is supported)", templates, nil)}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: waffle %s <type> <domain> <entity>", c.Code())
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
