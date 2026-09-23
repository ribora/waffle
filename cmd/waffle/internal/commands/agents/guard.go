package agents

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

func NewGuard() *guard {
	return &guard{base: command.New("agents:guard",
		"Exit non-zero, naming the mode or grant that would allow it, if the mode does not allow editing the path",
		noTemplates, nil).
		WithUsage("<mode> <path> [--grant <name>]...\n" +
			"  mode is one of: " + strings.Join(modeNames(), ", ") + "\n" +
			"  path is absolute or relative to the working directory\n" +
			"  --grant names an area the human granted on top of the mode\n" +
			"  a path outside this service that no area of it names prints \"unclaimed\" and exits 0")}
}

type guard struct {
	base *command.Command
}

func (c *guard) Code() string        { return c.base.Code() }
func (c *guard) Description() string { return c.base.Description() }
func (c *guard) Usage() string       { return c.base.Usage() }

func (c *guard) Run(_ context.Context, args ...string) error {
	var granted []string
	for i := slices.Index(args, "--grant"); i >= 0; i = slices.Index(args, "--grant") {
		if i+1 == len(args) {
			return errors.New("--grant takes an area's name")
		}
		granted = append(granted, args[i+1])
		args = slices.Delete(args, i, i+2)
	}
	if err := c.base.Operands(args, 2, 2); err != nil {
		return err
	}
	mode, err := LookupMode(args[0])
	if err != nil {
		return err
	}
	rules, err := serviceRules(c.base)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(args[1])
	if err != nil {
		return err
	}
	if abs, err = resolveExisting(abs); err != nil {
		return err
	}

	switch v, reason := rules.Check(mode, abs, granted); v {
	case Refused:
		return errors.New(reason)
	case Unclaimed:
		fmt.Printf("unclaimed: no area of this service names %s\n", abs)
	}
	return nil
}

func serviceRules(c *command.Command) (Rules, error) {
	root, err := command.ModuleRoot()
	if err != nil {
		return Rules{}, err
	}
	if root, err = filepath.EvalSymlinks(root); err != nil {
		return Rules{}, err
	}
	project, err := c.FindProjectName()
	if err != nil {
		return Rules{}, err
	}
	cfg, err := c.ProjectConfig(project)
	if err != nil {
		return Rules{}, err
	}
	areas, err := Areas(cfg.Agents)
	if err != nil {
		return Rules{}, err
	}
	return Rules{Root: root, Areas: areas}, nil
}

func resolveExisting(p string) (string, error) {
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			return filepath.Join(append([]string{resolved}, tail...)...), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return filepath.Join(append([]string{p}, tail...)...), nil
		}
		tail = append([]string{filepath.Base(p)}, tail...)
		p = parent
	}
}
