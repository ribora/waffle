package migrate

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"time"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/plural"
)

//go:embed templates/*
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("make:domain:migrate", "Set up migrations to the specified domain", templates, nil)}
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
	service := filepath.Base(projectName)

	params := map[string]string{
		"Domain":  domain,
		"Project": projectName,
		"Service": service,
	}

	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "entity", "main.go.tpl"):        filepath.Join(path, "cmd", "migrate", "main.go"),
		filepath.Join("templates", "entity", "updated_at.sql.tpl"): filepath.Join(path, "cmd", "migrate", "migrations", fmt.Sprintf("00000000000000_create_update_at_function.sql")),
		filepath.Join("templates", "entity", "migration.sql.tpl"):  filepath.Join(path, "cmd", "migrate", "migrations", fmt.Sprintf("%s_create_%s_table.sql", time.Now().Format("20060102150405"), plural.Plural(domain))),
		filepath.Join("templates", "di.go.tpl"):                    filepath.Join("config", "di", "di_"+domain+"_migrate.go"),
	}

	return c.Write(sources, params)
}
