package agents

import (
	"context"
	"fmt"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

func NewMode() *mode {
	return &mode{base: command.New("agents:mode",
		"Print what a Rule #11 mode allows and which part of AGENTS.md it follows; switches nothing").
		WithUsage("<mode>\n" +
			"  mode is one of: " + strings.Join(modeNames(), ", "))}
}

type mode struct {
	base *command.Command
}

func (c *mode) Code() string        { return c.base.Code() }
func (c *mode) Description() string { return c.base.Description() }
func (c *mode) Usage() string       { return c.base.Usage() }

func (c *mode) Run(_ context.Context, args ...string) error {
	if err := c.base.Operands(args, 1, 1); err != nil {
		return err
	}
	m, err := LookupMode(args[0])
	if err != nil {
		return err
	}
	fmt.Printf("Mode: %s\n%s\nMay be edited: %s.\n", m.Name, m.Brief, m.Allows)
	return nil
}
