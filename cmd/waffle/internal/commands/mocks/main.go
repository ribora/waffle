package mocks

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
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
		WithUsage("[--suggest | --write | --check] [--container=<path>] [--dir=<path>]\n" +
			"  --suggest  print the seams listed in neither mocks.generate nor mocks.ignore\n" +
			"  --write    add those to mocks.generate in waffle.yaml\n" +
			"  --check    exit non-zero if there are any, for CI\n" +
			"  --dir      write the mocks here instead of mocks.dir from waffle.yaml")}
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

	var dir string
	args = slices.DeleteFunc(args, func(a string) bool {
		value, ok := strings.CutPrefix(a, "--dir=")
		if ok {
			dir = value
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
	reporting := count(suggest, write, check)
	if reporting > 1 {
		return errors.New("--suggest, --write and --check do different things with the same list; pick one\n" + c.Usage())
	}
	if dir != "" && reporting > 0 {
		return errors.New("--dir only affects where mocks are written, and --suggest, --write and --check write none\n" + c.Usage())
	}

	project, err := c.FindProjectName()
	if err != nil {
		return err
	}
	cfg, err := c.ProjectConfig(project)
	if err != nil {
		return err
	}
	if dir != "" {
		resolved, err := insideProject(dir)
		if err != nil {
			return err
		}
		cfg.Mocks.Dir = resolved
	}

	if reporting == 0 {
		if staleDefaultDir(cfg, dir) {
			command.ReportWarning(oldDefaultDir + " is left over from the previous default and nothing writes to it — " +
				"delete it, or set mocks.dir in waffle.yaml to keep it")
		}
		return c.generate(cfg)
	}

	seams, err := readSeams(containers, project)
	if err != nil {
		return err
	}
	return c.report(seams, cfg, project, write, check)
}

// The default moved, and prune only knows the directory it writes to, so mocks
// under the old one survive: compiling, importable and stale.
const oldDefaultDir = "internal/dimock"

func staleDefaultDir(cfg command.Config, dir string) bool {
	if dir != "" || cfg.Mocks.Dir != filepath.Join("tests", "mocks") {
		return false
	}
	_, err := os.Stat(oldDefaultDir)
	return err == nil
}

// Outside the project this would generate into — and, with an empty
// mocks.generate, prune inside — somebody else's repository.
func insideProject(dir string) (string, error) {
	cleaned := filepath.Clean(dir)
	// filepath.Base(".") is "." — mockgen fails on it as a package name.
	if cleaned == "." {
		return "", errors.New("--dir needs a directory to write into, not the project root")
	}
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	absolute := cleaned
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(root, cleaned)
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("--dir %s is outside the project", dir)
	}
	return relative, nil
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

// Only bare keys count: rewriting the value of "mocks: {}" is how a config file
// gets eaten.
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
	// A decision, not an omission: every seam may be a spy or a real connection,
	// and what an earlier decision left behind still has to go.
	if len(cfg.Mocks.Generate) == 0 {
		removed, err := prune(cfg.Mocks.Dir, nil)
		if err != nil {
			return err
		}
		if removed == 0 {
			return errors.New("waffle.yaml lists nothing under mocks.generate — run with --suggest to see what the containers expose")
		}
		return nil
	}
	if err := c.EnsureTool(mockgenModule); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Mocks.Dir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create %s: %w", cfg.Mocks.Dir, err)
	}

	pkg := filepath.Base(cfg.Mocks.Dir)
	invocations := plan(cfg.Mocks.Generate, cfg.Mocks.Dir)
	for _, inv := range invocations {
		if out, err := exec.Command("go", inv.args(pkg)...).CombinedOutput(); err != nil {
			return fmt.Errorf("mockgen %s: %v: %s", inv.Path, err, string(out))
		}
		command.ReportCreated(inv.Destination)
	}
	_, err := prune(cfg.Mocks.Dir, invocations)
	return err
}

// An interface moved to mocks.ignore would otherwise leave its mock behind,
// compiling and importable. Only files mockgen wrote are candidates.
func prune(dir string, invocations []invocation) (int, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to read %s: %w", dir, err)
	}

	wanted := make(map[string]bool, len(invocations))
	for _, inv := range invocations {
		wanted[filepath.Base(inv.Destination)] = true
	}

	var removed int
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || wanted[name] {
			continue
		}
		file := filepath.Join(dir, name)
		generated, err := isGenerated(file)
		if err != nil {
			return removed, err
		}
		if !generated {
			continue
		}
		if err := os.Remove(file); err != nil {
			return removed, fmt.Errorf("failed to remove the stale mock %s: %w", file, err)
		}
		command.ReportRemoved("no longer in mocks.generate", file)
		removed++
	}
	return removed, nil
}

// Read as a prefix, not searched for: a file that merely mentions this line is
// not generated.
const generatedHeader = "// Code generated by MockGen. DO NOT EDIT."

func isGenerated(file string) (bool, error) {
	fh, err := os.Open(file)
	if err != nil {
		return false, fmt.Errorf("failed to read %s: %w", file, err)
	}
	defer func() { _ = fh.Close() }()

	head := make([]byte, len(generatedHeader))
	n, err := io.ReadFull(fh, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("failed to read %s: %w", file, err)
	}
	return string(head[:n]) == generatedHeader, nil
}
