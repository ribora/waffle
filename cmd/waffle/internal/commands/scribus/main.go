package scribus

import (
	"context"
	"embed"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("make:domain:scribus", "Make scribus handler for the specified domain (use --batch for BatchHandler)", templates, nil)}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(ctx context.Context, args ...string) error {
	batch := false
	if i := slices.Index(args, "--batch"); i >= 0 {
		batch = true
		args = append(args[:i], args[i+1:]...)
	}

	if len(args) < 2 {
		return fmt.Errorf("usage: waffle %s [--batch] <domain> <event>", c.Code())
	}
	domain := args[0]
	event := args[1]
	projectName, err := c.FindProjectName()
	if err != nil {
		return err
	}

	params := map[string]string{
		"Domain":  domain,
		"Event":   event,
		"Project": projectName,
	}

	mode := "single"
	includeTool := "gitlab.com/gorib/waffle/config/tools/scribus/single"
	if batch {
		mode = "batch"
		includeTool = "gitlab.com/gorib/waffle/config/tools/scribus/batch"
	}

	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", mode, "handler.go.tpl"):  filepath.Join(path, "cmd", "scribus", event+".go"),
		filepath.Join("templates", "model", "event.go.tpl"): filepath.Join(path, "cmd", "scribus", "model", event+".go"),
		filepath.Join("templates", mode, "di.go.tpl"):       filepath.Join("config", "di", "di_"+domain+"_"+event+"_scribus.go"),
	}

	if err := c.Write(sources, params); err != nil {
		return err
	}

	if err := c.UncommentModule(includeTool); err != nil {
		return err
	}

	info, err := exec.Command("go", "get", "-tool", "gitlab.com/gorib/gorival/cmd/gorival@latest").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	info, err = exec.Command("make", ".tidy", "generate").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %v", err, string(info))
	}
	return nil
}
