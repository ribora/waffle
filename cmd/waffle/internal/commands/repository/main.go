package repository

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/migrate"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

const defaultStorage = "sql"

func New() *cmd {
	return &cmd{command.New("make:domain:entity", "Add an entity with its repository to an existing domain", templates, nil).
		WithUsage("[--storage=<type>, default sql] <domain> <entity>")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	storage := defaultStorage
	for i, arg := range args {
		if after, found := strings.CutPrefix(arg, "--storage="); found {
			storage = after
			args = append(args[:i], args[i+1:]...)
			break
		}
	}
	if storage != defaultStorage {
		return fmt.Errorf("storage %q is not supported (only %s for now)", storage, defaultStorage)
	}

	if err := c.Operands(args, 2, 2); err != nil {
		return err
	}
	domain, entity := args[0], args[1]
	if err := command.ValidateName("domain", domain); err != nil {
		return err
	}
	if err := command.ValidateName("entity", entity); err != nil {
		return err
	}
	projectName, err := c.FindProjectName()
	if err != nil {
		return err
	}

	// The same three calls, in the same order, that make:domain makes for the
	// domain's own entity.
	if err := GenerateEntity(domain, entity, projectName); err != nil {
		return err
	}
	if err := migrate.Generate(domain, entity, projectName); err != nil {
		return err
	}
	if err := c.EnableStorage(); err != nil {
		return err
	}
	reportWiring(domain, entity)

	info, err := exec.Command("make", ".tidy", "generate").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	return nil
}

// Whether the repository belongs to the domain's management is not the
// scaffolder's to decide — a read model read only by a handler does not.
// diogen wires it by type as soon as the parameter is there.
func reportWiring(domain, entity string) {
	name := command.Title(entity)
	command.ReportBlock(
		fmt.Sprintf("to reach it from the management, add to New%sManagement in domain/%s/service/service.go:", command.Title(domain), domain),
		fmt.Sprintf("%sRepository %sRepository", entity, name),
	)
}
