package service

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
	return &cmd{command.New("init:service", "Initialize empty service", templates, nil)}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, _ ...string) error {
	projectName, err := c.FindProjectName()
	if err != nil {
		return err
	}
	service := filepath.Base(projectName)

	params := map[string]string{
		"Project": projectName,
		"Service": service,
	}

	sources := map[string]string{
		filepath.Join("templates", "api", "service", "init.go.tpl"):            filepath.Join("api", service, "init.go"),
		filepath.Join("templates", "api", "service", "proto", "gitignore.tpl"): filepath.Join("api", service, "proto", ".gitignore"),
		filepath.Join("templates", "cmd", "app", "main.go.tpl"):                filepath.Join("cmd", "app", "main.go"),
		filepath.Join("templates", "config", "config.go.tpl"):                  filepath.Join("config", "config.go"),
		filepath.Join("templates", "config", "di.go.tpl"):                      filepath.Join("config", "di", "di.go"),
		filepath.Join("templates", "config", "modules.go.tpl"):                 filepath.Join("config", "di", "modules.go"),
		filepath.Join("templates", "config", "diogen.go.tpl"):                  filepath.Join("config", "diogen", "di.go"),
		filepath.Join("templates", "gitignore.tpl"):                            filepath.Join(".gitignore"),
		filepath.Join("templates", "Makefile.tpl"):                             filepath.Join("Makefile"),
		filepath.Join("templates", "env.tpl"):                                  filepath.Join(".env"),
	}

	err = c.Write(sources, params)
	if err != nil {
		return err
	}
	info, err := exec.Command("go", "get", "-tool", "gitlab.com/gorib/di/cmd/diogen@latest").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	info, err = exec.Command("make", ".tidy", "generate").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	return nil
}
