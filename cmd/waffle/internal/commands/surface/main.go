package surface

import (
	"bytes"
	"context"
	"embed"
	"os"
	"path/filepath"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	"gitlab.com/gorib/waffle/surface"
)

var noTemplates embed.FS

func New() *cmd {
	return &cmd{command.New("surface",
		"Write tests/surface.yaml from this module's code: what the service publishes, consumes, serves and calls",
		noTemplates, nil)}
}

type cmd struct {
	*command.Command
}

func (c *cmd) Run(_ context.Context, args ...string) error {
	if err := c.Operands(args, 0, 0); err != nil {
		return err
	}
	root, err := command.ModuleRoot()
	if err != nil {
		return err
	}
	s, err := surface.Extract(root)
	if err != nil {
		return err
	}
	body := s.Encode()
	target := filepath.Join(root, surface.File)
	if current, err := os.ReadFile(target); err == nil && bytes.Equal(current, body) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(target, body, 0o644); err != nil {
		return err
	}
	command.ReportAction("wrote " + surface.File)
	return nil
}
