package agents

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

func NewGrant() *grant {
	return &grant{base: command.New("agents:grant",
		"Print the areas a Rule #11 mode takes as a grant, or check one and say what it opens; grants nothing",
		noTemplates, nil).
		WithUsage("<mode> [name]\n" +
			"  mode is one of: " + strings.Join(modeNames(), ", ") + "\n" +
			"  name is an area — the defaults, and any waffle.yaml declares under agents.areas")}
}

type grant struct {
	base *command.Command
}

func (c *grant) Code() string        { return c.base.Code() }
func (c *grant) Description() string { return c.base.Description() }
func (c *grant) Usage() string       { return c.base.Usage() }

func (c *grant) Run(_ context.Context, args ...string) error {
	if err := c.base.Operands(args, 1, 2); err != nil {
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

	if len(args) == 1 {
		var lines []string
		for _, a := range rules.Areas {
			if slices.Contains(a.Grant, mode.Name) {
				lines = append(lines, fmt.Sprintf("  %s — %s", a.Name, strings.Join(a.Paths, ", ")))
			}
		}
		if len(lines) == 0 {
			fmt.Printf("No area is granted in %s mode.\n", mode.Name)
			return nil
		}
		fmt.Printf("Grants in %s mode:\n%s\n", mode.Name, strings.Join(lines, "\n"))
		return nil
	}

	for _, a := range rules.Areas {
		if a.Name != args[1] {
			continue
		}
		if !slices.Contains(a.Grant, mode.Name) {
			return fmt.Errorf("the %s area is not granted in %s mode (it is in: %s)",
				a.Name, mode.Name, orNone(a.Grant))
		}
		fmt.Printf("Grant: %s, on top of %s mode, until the mode changes.\nMay also be edited: %s.\n",
			a.Name, mode.Name, strings.Join(a.Paths, ", "))
		return nil
	}
	return fmt.Errorf("no area is named %q", args[1])
}

func orNone(modes []string) string {
	if len(modes) == 0 {
		return "no mode"
	}
	return strings.Join(modes, ", ")
}
