package replacements

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	check "gitlab.com/gorib/waffle/replacements"
)

// No templates: this command reads and reports, it writes nothing.
var templates embed.FS

func New() *cmd {
	return &cmd{command.New("check:tests:replacements",
		"Check that the tests replace only what waffle.yaml allows",
		templates, nil).
		WithUsage("[--suggest] [--container=<path>]\n" +
			"  --suggest    print every seam the containers expose, marking what is already allowed\n" +
			"  --container  read this generated container instead of the ones //go:generate names")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(_ context.Context, args ...string) error {
	var containers []string
	args = slices.DeleteFunc(args, func(a string) bool {
		value, ok := strings.CutPrefix(a, "--container=")
		if ok {
			containers = append(containers, value)
		}
		return ok
	})

	suggest := takeFlag(&args, "--suggest")

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
	cfg, err := c.ProjectConfig(project)
	if err != nil {
		return err
	}
	root, err := command.ModuleRoot()
	if err != nil {
		return err
	}
	read, err := targets(root, containers)
	if err != nil {
		return err
	}
	found, err := check.Inspect(root, read)
	if err != nil {
		return err
	}

	if suggest {
		fmt.Print(suggestion(found.Seams, cfg, project))
		return nil
	}
	return report(check.Verify(root, found, cfg.Replacements))
}

func takeFlag(args *[]string, name string) bool {
	if i := slices.Index(*args, name); i >= 0 {
		*args = slices.Delete(*args, i, i+1)
		return true
	}
	return false
}

// A named container is taken at its word — that is what the flag is for.
func targets(root string, overrides []string) ([]check.Container, error) {
	if len(overrides) == 0 {
		return check.DiscoverContainers(root)
	}
	out := make([]check.Container, 0, len(overrides))
	for _, file := range overrides {
		// A path a person typed is relative to where they typed it; the check
		// works from the module root, which is not always the same place.
		if !filepath.IsAbs(file) {
			cwd, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("failed to get current working directory: %w", err)
			}
			file = filepath.Join(cwd, file)
		}
		dir := filepath.Dir(file)
		out = append(out, check.Container{File: file, Dir: dir, Package: filepath.Base(dir)})
	}
	return out, nil
}
