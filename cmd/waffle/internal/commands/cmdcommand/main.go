package cmdcommand

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
	return &cmd{command.New("make:domain:command", "Add a CLI command (app.Command) scoped to a domain", templates, nil).
		WithUsage("<domain> <command>")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	if err := c.Operands(args, 2, 2); err != nil {
		return err
	}
	if err := command.ValidateName("domain", args[0]); err != nil {
		return err
	}
	if err := command.ValidateName("command", args[1]); err != nil {
		return err
	}
	domain := args[0]
	cmdName := args[1]
	projectName, err := c.FindProjectName()
	if err != nil {
		return err
	}

	params := map[string]string{
		"Domain":  domain,
		"Command": cmdName,
		"Project": projectName,
	}

	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "main.go.tpl"): filepath.Join(path, "cmd", cmdName, "main.go"),
		filepath.Join("templates", "di.go.tpl"):   filepath.Join("config", "di", "di_"+domain+"_"+cmdName+".go"),
	}

	if err := c.Write(sources, params); err != nil {
		return err
	}

	info, err := exec.Command("make", ".tidy", "generate").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	return nil
}
