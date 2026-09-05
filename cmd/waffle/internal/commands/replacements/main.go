package replacements

import (
	"context"
	"embed"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
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
	seams, generated, err := readSeams(containers, project)
	if err != nil {
		return err
	}

	if suggest {
		fmt.Print(suggestion(seams, cfg, project))
		return nil
	}
	return check(seams, generated, cfg)
}

func takeFlag(args *[]string, name string) bool {
	if i := slices.Index(*args, name); i >= 0 {
		*args = slices.Delete(*args, i, i+1)
		return true
	}
	return false
}

// generated is what the check needs beyond the seams: the import paths that
// name the struct, and the files that declare it.
type generated struct {
	Paths []string
	Files []string
}

func readSeams(overrides []string, project string) ([]command.Seam, generated, error) {
	var containers []command.Container
	switch {
	case len(overrides) > 0:
		for _, file := range overrides {
			dir := filepath.Dir(file)
			containers = append(containers, command.Container{
				File: file, Dir: dir, Package: filepath.Base(dir),
			})
		}
	default:
		found, err := command.DiscoverContainers()
		if err != nil {
			return nil, generated{}, err
		}
		containers = found
	}

	var all []command.Seam
	var found generated
	for _, container := range containers {
		importPath := path.Join(project, filepath.ToSlash(container.Dir))
		seams, err := command.ReadSeams(container, importPath)
		if err != nil {
			return nil, generated{}, err
		}
		file, err := filepath.Abs(container.File)
		if err != nil {
			return nil, generated{}, err
		}
		all = append(all, seams...)
		found.Paths = append(found.Paths, importPath)
		found.Files = append(found.Files, file)
	}
	return all, found, nil
}
