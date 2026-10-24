package migration

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templateFiles embed.FS

var templates = command.NewTemplates(templateFiles)

func New() *cmd {
	return &cmd{command.New("make:domain:migration", "Add a new .sql migration to an existing domain").
		WithUsage("<domain> <description>")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	if err := c.Operands(args, 2, 2); err != nil {
		return err
	}
	if err := command.ValidateName("domain", args[0]); err != nil {
		return err
	}
	// Only ever a file name, so dashes and dots are fine and a separator is not.
	if err := command.ValidateFileName("description", args[1]); err != nil {
		return err
	}
	domain := args[0]
	name := args[1]
	p, err := command.OpenProject()
	if err != nil {
		return err
	}
	if err := p.RequireDomain(domain); err != nil {
		return err
	}
	// The runner is what applies the file; one written without it is never run.
	runner := filepath.Join("domain", domain, "cmd", "migrate", "main.go")
	if _, err := os.Stat(p.Path(runner)); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s does not exist — set up the domain's migrations first: waffle make:domain:migrate %s", runner, domain)
	} else if err != nil {
		return err
	}

	target := filepath.Join("domain", domain, "cmd", "migrate", "migrations",
		fmt.Sprintf("%s_%s.sql", time.Now().Format("20060102150405"), name))

	sources := map[string]string{
		filepath.Join("templates", "migration.sql.tpl"): target,
	}
	if err := p.Write(templates, sources, nil); err != nil {
		return err
	}
	fmt.Printf("Created %s\n", target)
	return nil
}
