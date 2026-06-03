package http

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
	return &cmd{command.New("make:domain:http", "Make HTTP router for the specified domain", templates, nil)}
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
		filepath.Join("templates", "main.go.tpl"): filepath.Join(path, "cmd", "http", "main.go"),
		filepath.Join("templates", "di.go.tpl"):   filepath.Join("config", "di", "di_"+domain+"_http.go"),
	}

	if err := c.Write(sources, params); err != nil {
		return err
	}

	if err := c.UncommentModule("gitlab.com/gorib/waffle/config/tools/http"); err != nil {
		return err
	}

	info, err := exec.Command("make", ".tidy", "generate").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	return nil
}
