package surface

import (
	"context"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	"gitlab.com/gorib/waffle/surface"
)

func New() *cmd {
	return &cmd{command.New("surface",
		"Write tests/surface.yaml from this module's code: what the service publishes, consumes, serves and calls")}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(_ context.Context, args ...string) error {
	if err := c.Operands(args, 0, 0); err != nil {
		return err
	}
	p, err := command.OpenProject()
	if err != nil {
		return err
	}
	s, err := surface.Extract(p.Root())
	if err != nil {
		return err
	}
	if changed, err := command.WriteIfChanged(p.Path(surface.File), s.Encode()); err != nil || !changed {
		return err
	}
	command.ReportAction("wrote " + surface.File)
	return nil
}
