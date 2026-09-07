package component

import (
	"context"
	"embed"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	"gitlab.com/gorib/waffle/replacements"
)

//go:embed templates/*
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("tests:container",
		"Report the container's own tests, and write the missing ones with init — no infrastructure",
		templates, nil).
		WithUsage("[init]\n" +
			"  init  write the tests that are not there yet, leaving the rest alone")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(_ context.Context, args ...string) error {
	if err := c.Operands(args, 0, 1); err != nil {
		return err
	}
	write := len(args) == 1
	if write && args[0] != "init" {
		return fmt.Errorf("unknown subcommand %q\n%s", args[0], c.Usage())
	}

	containers, err := replacements.DiscoverContainers(".")
	if err != nil {
		return err
	}
	if write {
		return c.scaffold(containers)
	}
	return c.state(containers)
}

func (c *cmd) scaffold(containers []replacements.Container) error {
	project, err := c.FindProjectName()
	if err != nil {
		return err
	}
	for i, container := range containers {
		params := map[string]string{
			"Package": container.Package,
			"Import":  path.Join(project, filepath.ToSlash(container.Dir)),
		}
		if err := c.Write(plan(i, container), params); err != nil {
			return err
		}
	}
	return nil
}

func (c *cmd) state(containers []replacements.Container) error {
	var present, absent []string
	for i, container := range containers {
		for _, target := range slices.Sorted(maps.Values(plan(i, container))) {
			if _, err := os.Stat(target); err == nil {
				present = append(present, target)
				continue
			}
			absent = append(absent, target)
		}
	}
	command.ReportKept(present...)
	command.ReportMissing(absent...)
	if len(absent) > 0 {
		command.ReportWarning("run `waffle tests:container init` to write them")
	}
	return nil
}

// One plan for both modes, so what a bare run reports is what init would write.
func plan(i int, container replacements.Container) map[string]string {
	sources := map[string]string{
		filepath.Join("templates", "app_test.go.tpl"): filepath.Join(container.Dir, "app_test.go"),
	}
	// The allow-list check reads the whole module, so one copy answers for every
	// container. It goes beside the first.
	if i == 0 {
		sources[filepath.Join("templates", "replacements_test.go.tpl")] =
			filepath.Join(container.Dir, "replacements_test.go")
	}
	return sources
}
