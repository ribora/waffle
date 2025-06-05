package service

import (
	"context"
	"embed"
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
		filepath.Join("templates", "gitlab", "auto-deploy-values.yaml.tpl"):      filepath.Join(".gitlab", "auto-deploy-values.yaml"),
		filepath.Join("templates", "api", "service", "init.go.tpl"):              filepath.Join("api", service, "init.go"),
		filepath.Join("templates", "api", "service", "pb", "gitignore.tpl"):      filepath.Join("api", service, "pb", ".gitignore"),
		filepath.Join("templates", "cmd", "app", "main.go.tpl"):                  filepath.Join("cmd", "app", "main.go"),
		filepath.Join("templates", "config", "config.go.tpl"):                    filepath.Join("config", "config.go"),
		filepath.Join("templates", "config", "di.go.tpl"):                        filepath.Join("config", "di.go"),
		filepath.Join("templates", "tools", "migrate", "main.go.tpl"):            filepath.Join("tools", "migrate", "main.go"),
		filepath.Join("templates", "tools", "migrate", "migrations", "keep.tpl"): filepath.Join("tools", "migrate", "migrations", ".keep"),
		filepath.Join("templates", "gitignore.tpl"):                              filepath.Join(".gitignore"),
		filepath.Join("templates", "gitlab-ci.yml.tpl"):                          filepath.Join(".gitlab-ci.yml"),
		filepath.Join("templates", "Makefile.tpl"):                               filepath.Join("Makefile"),
		filepath.Join("templates", "env.tpl"):                                    filepath.Join(".env"),
	}

	return c.Write(sources, params)
}
