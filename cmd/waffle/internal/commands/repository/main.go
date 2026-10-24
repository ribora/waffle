package repository

import (
	"context"
	"fmt"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/migrate"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

const defaultStorage = "sql"

func New() *cmd {
	return &cmd{command.New("make:domain:entity", "Add an entity with its repository to an existing domain").
		WithUsage("[--storage=<type>, default sql] <domain> <entity>")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	storages, args, err := command.TakeValues(args, "--storage", "the adapter's type, e.g. --storage sql")
	if err != nil {
		return err
	}
	for _, storage := range storages {
		if storage != defaultStorage {
			return fmt.Errorf("storage %q is not supported (only %s for now)", storage, defaultStorage)
		}
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
	p, err := command.OpenProject()
	if err != nil {
		return err
	}
	if err := p.RequireDomain(domain); err != nil {
		return err
	}
	if err := p.RequireModules(command.MigrationModules...); err != nil {
		return err
	}

	// The same three calls, in the same order, that make:domain makes for the
	// domain's own entity.
	if err := GenerateEntity(p, domain, entity); err != nil {
		return err
	}
	if err := migrate.Generate(p, domain, entity); err != nil {
		return err
	}
	if err := p.EnableStorage(); err != nil {
		return err
	}
	// Not `make generate`, for make:domain's reason: until the management reads
	// the repository, diogen rightly refuses the container.
	for _, step := range [][]string{
		{"make", ".tidy"},
		{"go", "generate", "./domain/" + domain + "/..."},
	} {
		if _, err := p.Run(step[0], step[1:]...); err != nil {
			return err
		}
	}
	reportWiring(domain, entity)
	return nil
}

// Whether the repository belongs to the domain's management is not the
// scaffolder's to decide — a read model read only by a handler does not.
// diogen wires it by type as soon as the parameter is there.
func reportWiring(domain, entity string) {
	name := command.Title(entity)
	command.ReportBlock(
		fmt.Sprintf("diogen refuses the container until something reads the repository — to reach it from the management, add to New%sManagement in domain/%s/service/service.go, then run make generate:", command.Title(domain), domain),
		fmt.Sprintf("%sRepository %sRepository", entity, name),
	)
}
