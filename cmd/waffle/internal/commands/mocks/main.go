package mocks

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

// No templates: mockgen writes the files, this command only decides what to ask
// it for.
var templates embed.FS

const mockgenModule = "go.uber.org/mock/mockgen"

func New() *cmd {
	return &cmd{command.New("make:tests:mocks",
		"Generate the mocks listed in waffle.yaml; --suggest lists what the container exposes",
		templates, nil).
		WithUsage("[--suggest | --write | --check] [--container=<path>]\n" +
			"  --suggest  print the seams listed in neither mocks.generate nor mocks.ignore\n" +
			"  --write    add those to mocks.generate in waffle.yaml\n" +
			"  --check    exit non-zero if there are any, for CI")}
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
	write := takeFlag(&args, "--write")
	check := takeFlag(&args, "--check")

	if err := command.RejectFlags(args); err != nil {
		return fmt.Errorf("%w\n%s", err, c.Usage())
	}
	if len(args) > 0 {
		return fmt.Errorf("unexpected operand %q\n%s", args[0], c.Usage())
	}
	if chosen := count(suggest, write, check); chosen > 1 {
		return errors.New("--suggest, --write and --check do different things with the same list; pick one\n" + c.Usage())
	}

	project, err := c.FindProjectName()
	if err != nil {
		return err
	}
	cfg, err := c.ProjectConfig(project)
	if err != nil {
		return err
	}

	if count(suggest, write, check) == 0 {
		return c.generate(cfg)
	}

	seams, err := readSeams(containers, project)
	if err != nil {
		return err
	}
	return c.report(seams, cfg, project, write, check)
}

func takeFlag(args *[]string, name string) bool {
	if i := slices.Index(*args, name); i >= 0 {
		*args = slices.Delete(*args, i, i+1)
		return true
	}
	return false
}

func count(flags ...bool) int {
	n := 0
	for _, set := range flags {
		if set {
			n++
		}
	}
	return n
}

func readSeams(overrides []string, project string) ([]command.Seam, error) {
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
			return nil, err
		}
		containers = found
	}

	// One interface is easily a seam of two containers; the report counts types,
	// so the union is what matters and duplicates are dropped downstream.
	var all []command.Seam
	for _, container := range containers {
		seams, err := command.ReadSeams(container, path.Join(project, filepath.ToSlash(container.Dir)))
		if err != nil {
			return nil, err
		}
		all = append(all, seams...)
	}
	return all, nil
}

func (c *cmd) report(seams []command.Seam, cfg command.Config, project string, write, check bool) error {
	missing := unlisted(seams, cfg)

	switch {
	case len(missing) == 0 && check:
		return nil
	case len(missing) == 0:
		fmt.Println("# every seam is listed in mocks.generate or mocks.ignore")
		return nil
	case check:
		return fmt.Errorf("%d packages expose seams that waffle.yaml lists in neither mocks.generate nor mocks.ignore:\n%s",
			packages(missing), strings.Join(entries(missing, project, "    "), "\n"))
	case write:
		return addToConfig(missing, project)
	default:
		fmt.Print(suggestion(missing, composites(seams), project))
		return nil
	}
}

// Inserted as text: a yaml.Node round-trip would reformat the prose at the top of
// waffle.yaml, which is half of what that file is for.
func addToConfig(missing []candidate, project string) error {
	root, err := command.ModuleRoot()
	if err != nil {
		return err
	}
	file := filepath.Join(root, "waffle.yaml")
	body, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", file, err)
	}

	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	added := entries(missing, project, "    ")

	switch mocks, generate := findKeys(lines); {
	case generate >= 0:
		lines = slices.Insert(lines, generate+1, added...)
	case mocks >= 0:
		lines = slices.Insert(lines, mocks+1, append([]string{"  generate:"}, added...)...)
	default:
		if len(lines) > 0 && lines[0] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, "mocks:", "  generate:")
		lines = append(lines, added...)
	}

	if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", file, err)
	}
	command.ReportCreated(fmt.Sprintf("waffle.yaml: %d packages added to mocks.generate", packages(missing)))
	return nil
}

// Where "mocks:" sits, and "generate:" inside its block. Only bare keys count:
// "mocks: {}" or a flow mapping would need its value rewritten, and guessing at
// that is how a config file gets eaten.
func findKeys(lines []string) (mocks, generate int) {
	mocks, generate = -1, -1
	for i, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		switch {
		case trimmed == "mocks:":
			mocks = i
		case mocks < 0:
			continue
		case trimmed == "  generate:":
			return mocks, i
		// The next top-level key ends the mocks block.
		case topLevelKey(trimmed):
			return mocks, -1
		}
	}
	return mocks, generate
}

func topLevelKey(trimmed string) bool {
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return false
	}
	return !strings.HasPrefix(trimmed, " ")
}

func (c *cmd) generate(cfg command.Config) error {
	if len(cfg.Mocks.Generate) == 0 {
		return errors.New("waffle.yaml lists nothing under mocks.generate — run with --suggest to see what the containers expose")
	}
	if err := c.EnsureTool(mockgenModule); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Mocks.Dir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create %s: %w", cfg.Mocks.Dir, err)
	}

	pkg := filepath.Base(cfg.Mocks.Dir)
	for _, inv := range plan(cfg.Mocks.Generate, cfg.Mocks.Dir) {
		if out, err := exec.Command("go", inv.args(pkg)...).CombinedOutput(); err != nil {
			return fmt.Errorf("mockgen %s: %v: %s", inv.Path, err, string(out))
		}
		command.ReportCreated(inv.Destination)
	}
	return nil
}
