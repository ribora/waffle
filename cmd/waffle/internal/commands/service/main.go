package service

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ribora/waffle/cmd/waffle/internal/commands/agents"
	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templateFiles embed.FS

var templates = command.NewTemplates(templateFiles)

func New() *cmd {
	return &cmd{command.New("init:service", "Initialize empty service")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	if err := c.Operands(args, 0, 0); err != nil {
		return err
	}
	p, err := command.OpenProject()
	if err != nil {
		return err
	}

	params := map[string]string{
		"Project": p.Module(),
		"Service": filepath.Base(p.Module()),
	}

	sources := map[string]string{
		filepath.Join("templates", "api", "init.go.tpl"):               filepath.Join("api", "init.go"),
		filepath.Join("templates", "api", "proto", "gitignore.tpl"):    filepath.Join("api", "proto", ".gitignore"),
		filepath.Join("templates", "cmd", "app", "main.go.tpl"):        filepath.Join("cmd", "app", "main.go"),
		filepath.Join("templates", "config", "config.go.tpl"):          filepath.Join("config", "config.go"),
		filepath.Join("templates", "config", "di.go.tpl"):              filepath.Join("config", "di", "di.go"),
		filepath.Join("templates", "config", "modules.go.tpl"):         command.ModulesFile,
		filepath.Join("templates", "config", "diogen.go.tpl"):          filepath.Join("config", "diogen", "di.go"),
		filepath.Join("templates", "config", "boundaries_test.go.tpl"): filepath.Join("config", "diogen", "boundaries_test.go"),
		filepath.Join("templates", "gitignore.tpl"):                    filepath.Join(".gitignore"),
		filepath.Join("templates", "Makefile.tpl"):                     filepath.Join("Makefile"),
		filepath.Join("templates", "env.tpl"):                          filepath.Join(".env"),
		filepath.Join("templates", "waffle.tpl"):                       "waffle",
		filepath.Join("templates", "waffle.yaml.tpl"):                  "waffle.yaml",
	}

	if err := p.Write(templates, sources, params); err != nil {
		return err
	}
	if err := os.Chmod(p.Path("waffle"), 0755); err != nil {
		return fmt.Errorf("failed to chmod waffle: %w", err)
	}
	if _, err := agents.Generate(p, false); err != nil {
		return err
	}
	if err := p.EnsureBuiltTool("github.com/ribora/diogen/cmd/diogen", "github.com/ribora/diogen/di"); err != nil {
		return err
	}
	if err := p.EnsureBuiltTool("github.com/ribora/waffle/cmd/waffle", "github.com/ribora/waffle"); err != nil {
		return err
	}
	_, err = p.Run("make", ".tidy", "generate")
	return err
}
