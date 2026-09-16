package domain

import (
	"context"
	"embed"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/migrate"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/repository"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("make:domain", "Make domain with entity, repository and management", templates, nil).
		WithUsage("[--no-repository] <domain>")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	// The model is written either way — what the flag skips is the repository,
	// and with it the table and the modules that serve one.
	noRepository := false
	if i := slices.Index(args, "--no-repository"); i >= 0 {
		noRepository = true
		args = append(args[:i], args[i+1:]...)
	}

	if err := c.Operands(args, 1, 1); err != nil {
		return err
	}
	domain := args[0]
	if err := command.ValidateName("domain", domain); err != nil {
		return err
	}
	projectName, err := c.FindProjectName()
	if err != nil {
		return err
	}
	// Before the first write: eleven files and a go.mod edit stand between here
	// and UncommentModule.
	if !noRepository {
		if err := c.RequireModules(); err != nil {
			return err
		}
	}

	params := map[string]any{
		"Domain":         domain,
		"Project":        projectName,
		"WithRepository": !noRepository,
	}

	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "model", "errors.go.tpl"):   filepath.Join(path, "model", "errors.go"),
		filepath.Join("templates", "service", "entity.go.tpl"): filepath.Join(path, "service", "service.go"),
		filepath.Join("templates", "di.go.tpl"):                filepath.Join("config", "di", "di_"+domain+"_domain.go"),
	}
	if err := c.Write(sources, params); err != nil {
		return err
	}

	// The domain's own entity goes through the same path as every later one, so
	// the first entity and the second differ in nothing.
	switch {
	case noRepository:
		if err := repository.WriteModel(domain, domain, projectName); err != nil {
			return err
		}
	default:
		if err := repository.GenerateEntity(domain, domain, projectName); err != nil {
			return err
		}
		if err := migrate.Generate(domain, domain, projectName); err != nil {
			return err
		}
		if err := c.EnableStorage(); err != nil {
			return err
		}
	}
	info, err := exec.Command("make", ".tidy", "generate").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	return nil
}
