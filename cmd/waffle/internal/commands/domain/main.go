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
	return &cmd{command.New("make:domain", "Make domain with entity, repository and management", templates, nil)}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	noStorage := false
	if i := slices.Index(args, "--no-storage"); i >= 0 {
		noStorage = true
		args = append(args[:i], args[i+1:]...)
	}

	if len(args) == 0 {
		return fmt.Errorf("usage: waffle %s [--no-storage] <domain>", c.Code())
	}
	domain := args[0]
	projectName, err := c.FindProjectName()
	if err != nil {
		return err
	}

	params := map[string]any{
		"Domain":      domain,
		"Project":     projectName,
		"WithStorage": !noStorage,
	}

	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "model", "entity.go.tpl"):   filepath.Join(path, "model", domain+".go"),
		filepath.Join("templates", "model", "errors.go.tpl"):   filepath.Join(path, "model", "errors.go"),
		filepath.Join("templates", "service", "entity.go.tpl"): filepath.Join(path, "service", domain+".go"),
		filepath.Join("templates", "di.go.tpl"):                filepath.Join("config", "di", "di_"+domain+"_domain.go"),
	}
	if err := c.Write(sources, params); err != nil {
		return err
	}

	if !noStorage {
		if err := repository.GenerateForDomain(domain, projectName); err != nil {
			return err
		}
		if err := migrate.Generate(domain, projectName); err != nil {
			return err
		}
		info, err := exec.Command("go", "get", "-tool", "gitlab.com/gorib/sql/cmd/store@latest").CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %v", err, string(info))
		}
		if err := c.UncommentModule("gitlab.com/gorib/waffle/config/service/sql"); err != nil {
			return err
		}
		if err := c.UncommentModule("gitlab.com/gorib/waffle/config/tools/migrate"); err != nil {
			return err
		}
	}
	info, err := exec.Command("make", ".tidy", "generate").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	return nil
}
