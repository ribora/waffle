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
		"Print the areas a Rule #11 mode opens on their own and those it takes as a grant, or check one grant and say what it opens; grants nothing").
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
	rules, err := serviceRules()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		fmt.Print(listing(mode, rules))
		return nil
	}

	for _, a := range rules.Areas {
		if a.Name != args[1] {
			continue
		}
		paths := rules.claimed(a)
		if len(paths) == 0 {
			return fmt.Errorf("the %s area opens nothing: another area names each of its paths alike and decides there", a.Name)
		}
		if slices.Contains(a.Modes, mode.Name) {
			return fmt.Errorf("the %s area is open in %s mode already and takes no grant there", a.Name, mode.Name)
		}
		if !slices.Contains(a.Grant, mode.Name) {
			return fmt.Errorf("the %s area is not granted in %s mode (it is in: %s)",
				a.Name, mode.Name, orNone(a.Grant))
		}
		fmt.Printf("Grant: %s, on top of %s mode, until the mode changes.\nMay also be edited: %s.\n",
			a.Name, mode.Name, strings.Join(paths, ", "))
		return nil
	}
	return fmt.Errorf("no area is named %q", args[1])
}

// Areas that follow the mode table are left to `agents:mode`, which prints what
// the table opens.
func listing(mode Mode, rules Rules) string {
	var open, grants []string
	for _, a := range rules.Areas {
		paths := rules.claimed(a)
		if len(paths) == 0 {
			continue
		}
		line := fmt.Sprintf("  %s — %s", a.Name, strings.Join(paths, ", "))
		if slices.Contains(a.Modes, mode.Name) {
			open = append(open, line)
		}
		if slices.Contains(a.Grant, mode.Name) {
			grants = append(grants, line)
		}
	}
	var b strings.Builder
	if len(open) > 0 {
		fmt.Fprintf(&b, "Open in %s mode on their own:\n%s\n", mode.Name, strings.Join(open, "\n"))
	}
	if len(grants) == 0 {
		fmt.Fprintf(&b, "No area takes a grant in %s mode.\n", mode.Name)
	} else {
		fmt.Fprintf(&b, "Grants in %s mode:\n%s\n", mode.Name, strings.Join(grants, "\n"))
	}
	return b.String()
}

func orNone(modes []string) string {
	if len(modes) == 0 {
		return "no mode"
	}
	return strings.Join(modes, ", ")
}
