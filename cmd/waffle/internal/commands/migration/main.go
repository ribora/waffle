package migration

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"time"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("make:domain:migration", "Add a new .sql migration to an existing domain", templates, nil)}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: waffle %s <domain> <name>", c.Code())
	}
	domain := args[0]
	name := args[1]

	target := filepath.Join("domain", domain, "cmd", "migrate", "migrations",
		fmt.Sprintf("%s_%s.sql", time.Now().Format("20060102150405"), name))

	sources := map[string]string{
		filepath.Join("templates", "migration.sql.tpl"): target,
	}
	if err := c.Write(sources, nil); err != nil {
		return err
	}
	fmt.Printf("Created %s\n", target)
	return nil
}
