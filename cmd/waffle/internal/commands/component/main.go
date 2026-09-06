package component

import (
	"context"
	"embed"
	"fmt"
	"path"
	"path/filepath"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	"gitlab.com/gorib/waffle/replacements"
)

//go:embed templates/*
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("make:tests:component",
		"Scaffold the container's own tests next to InitAppWith — no infrastructure", templates, nil)}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(_ context.Context, args ...string) error {
	if err := command.RejectFlags(args); err != nil {
		return fmt.Errorf("%w\n%s", err, c.Usage())
	}
	if len(args) > 0 {
		return fmt.Errorf("unexpected operand %q\n%s", args[0], c.Usage())
	}

	project, err := c.FindProjectName()
	if err != nil {
		return err
	}
	containers, err := replacements.DiscoverContainers(".")
	if err != nil {
		return err
	}

	for i, container := range containers {
		params := map[string]string{
			"Package": container.Package,
			"Import":  path.Join(project, filepath.ToSlash(container.Dir)),
		}
		sources := map[string]string{
			filepath.Join("templates", "app_test.go.tpl"): filepath.Join(container.Dir, "app_test.go"),
		}
		// The allow-list check reads the whole module, so one copy answers for
		// every container. It goes beside the first.
		if i == 0 {
			sources[filepath.Join("templates", "replacements_test.go.tpl")] =
				filepath.Join(container.Dir, "replacements_test.go")
		}
		if err := c.Write(sources, params); err != nil {
			return err
		}
	}
	return nil
}
