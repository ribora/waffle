package http

import (
	"context"
	"embed"
	"path/filepath"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/converter"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/whisk"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templateFiles embed.FS

var templates = command.NewTemplates(templateFiles)

const module = "gitlab.com/gorib/waffle/config/tools/http"

func New() *cmd {
	return &cmd{command.New("make:domain:http", "Make HTTP router for the specified domain").
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
	p, err := command.OpenProject()
	if err != nil {
		return err
	}
	if err := p.RequireDomain(domain); err != nil {
		return err
	}
	if err := p.RequireModules(module); err != nil {
		return err
	}
	cfg, err := p.Config()
	if err != nil {
		return err
	}

	params := map[string]string{
		"Domain":  domain,
		"Project": p.Module(),
	}

	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "main.go.tpl"): filepath.Join(path, "cmd", "http", "main.go"),
		filepath.Join("templates", "di.go.tpl"):   filepath.Join("config", "di", "di_"+domain+"_http.go"),
	}

	if err := p.Write(templates, sources, params); err != nil {
		return err
	}
	if cfg.Mapper == "goverter" {
		if err := converter.Generate(p, filepath.Join(path, "cmd", "http", "convert")); err != nil {
			return err
		}
	}

	if err := p.EnableModule(module); err != nil {
		return err
	}

	// Request DTOs in cmd/http are whisk-generated, and the author writes the
	// go:generate line by hand — so the tool has to be in place before they do.
	if err := whisk.Ensure(p); err != nil {
		return err
	}

	_, err = p.Run("make", ".tidy", "generate")
	return err
}
