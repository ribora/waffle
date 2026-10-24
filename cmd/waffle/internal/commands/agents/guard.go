package agents

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

func NewGuard() *guard {
	return &guard{base: command.New("agents:guard",
		"Exit non-zero, naming the mode or grant that would allow it, if the mode does not allow editing the path").
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
	granted, args, err := command.TakeValues(args, "--grant", "an area's name")
	if err != nil {
		return err
	}
	if err := c.base.Operands(args, 2, 2); err != nil {
		return err
	}
	mode, err := LookupMode(args[0])
	if err != nil {
		return err
	}
	rules, err := serviceRules()
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(args[1])
	if err != nil {
		return err
	}
	resolved, err := resolveExisting(abs)
	if err != nil {
		return err
	}

	verdict := Unclaimed
	for _, p := range []string{rules.written(abs), resolved} {
		switch v, reason := rules.Check(mode, p, granted); v {
		case Refused:
			return errors.New(reason)
		case Allowed:
			verdict = Allowed
		}
	}
	if verdict == Unclaimed {
		fmt.Printf("unclaimed: no area of this service names %s\n", resolved)
	}
	return nil
}

func serviceRules() (Rules, error) {
	p, err := command.OpenProject()
	if err != nil {
		return Rules{}, err
	}
	root, err := filepath.EvalSymlinks(p.Root())
	if err != nil {
		return Rules{}, err
	}
	cfg, err := p.Config()
	if err != nil {
		return Rules{}, err
	}
	areas, err := Areas(cfg)
	if err != nil {
		return Rules{}, err
	}
	return newRules(root, areas)
}

func resolveExisting(p string) (string, error) {
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			return filepath.Join(append([]string{onDisk(resolved)}, tail...)...), nil
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

func onDisk(p string) string {
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	dir := onDisk(parent)
	return filepath.Join(dir, listed(dir, filepath.Base(p)))
}

// A case-insensitive filesystem finds a name under any spelling; only the
// listing has the one the rules compare against.
func listed(dir, name string) string {
	if _, err := os.Lstat(filepath.Join(dir, name)); err != nil {
		return name
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return name
	}
	spelled := name
	for _, e := range entries {
		if e.Name() == name {
			return name
		}
		if strings.EqualFold(e.Name(), name) {
			spelled = e.Name()
		}
	}
	return spelled
}
