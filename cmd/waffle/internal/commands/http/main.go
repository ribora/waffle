package http

import (
	"context"
	"embed"
	"fmt"
	"os/exec"
	"path/filepath"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/converter"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/whisk"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("make:domain:http", "Make HTTP router for the specified domain", templates, nil).
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
	projectName, err := c.FindProjectName()
	if err != nil {
		return err
	}

	cfg, err := c.ProjectConfig(projectName)
	if err != nil {
		return err
	}

	params := map[string]string{
		"Domain":  domain,
		"Project": projectName,
	}

	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "main.go.tpl"): filepath.Join(path, "cmd", "http", "main.go"),
		filepath.Join("templates", "di.go.tpl"):   filepath.Join("config", "di", "di_"+domain+"_http.go"),
	}

	if err := c.Write(sources, params); err != nil {
		return err
	}
	if cfg.Mapper == "goverter" {
		if err := converter.Generate(filepath.Join(path, "cmd", "http", "convert"), projectName); err != nil {
			return err
		}
	}

	if err := c.UncommentModule("gitlab.com/gorib/waffle/config/tools/http"); err != nil {
		return err
	}

	// Request DTOs in cmd/http are whisk-generated, and the author writes the
	// go:generate line by hand — so the tool has to be in place before they do.
	if err := whisk.Ensure(c.Command); err != nil {
		return err
	}

	info, err := exec.Command("make", ".tidy", "generate").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	return nil
}
