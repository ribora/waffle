package domain

import (
	"context"
	"embed"
	"fmt"
	"os/exec"
	"path/filepath"

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
	if len(args) == 0 {
		return fmt.Errorf("usage: waffle %s <domain>", c.Code())
	}
	domain := args[0]
	projectName, err := c.FindProjectName()
	if err != nil {
		return err
	}

	params := map[string]string{
		"Domain":  domain,
		"Project": projectName,
	}

	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "model", "entity.go.tpl"):               filepath.Join(path, "model", domain+".go"),
		filepath.Join("templates", "model", "errors.go.tpl"):               filepath.Join(path, "model", "errors.go"),
		filepath.Join("templates", "adapter", "postgres", "entity.go.tpl"): filepath.Join(path, "adapter", "postgres", domain+".go"),
		filepath.Join("templates", "adapter", "postgres", "stored.go.tpl"): filepath.Join(path, "adapter", "postgres", "stored_"+domain+".go"),
		filepath.Join("templates", "service", "entity.go.tpl"):             filepath.Join(path, "service", domain+".go"),
		filepath.Join("templates", "di.go.tpl"):                            filepath.Join("config", "di", "di_"+domain+"_domain.go"),
	}

	err = c.Write(sources, params)
	if err != nil {
		return err
	}
	info, err := exec.Command("go", "get", "-tool", "gitlab.com/gorib/storage/sql/cmd/store@latest").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	info, err = exec.Command("make", ".tidy", "generate").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	return nil
}
