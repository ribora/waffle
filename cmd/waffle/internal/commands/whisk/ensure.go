// Package whisk wires the whisk generator into a project. Shared, because every
// command that scaffolds a layer with whisk-tagged DTOs needs the tool in place
// before the first `//go:generate go tool whisk` the author writes.
package whisk

import (
	"os"

	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command"
)

const module = "github.com/ribora/whisk/cmd/whisk"

func Ensure(p *command.Project) error {
	if err := p.EnsureTool(module); err != nil {
		return err
	}
	// whisk writes whisk.yaml itself rather than waffle carrying a second copy of
	// its defaults; it leaves an existing file alone and reports which happened.
	report, err := p.Run("go", "tool", "whisk", "init")
	if err != nil {
		return err
	}
	_, _ = os.Stderr.Write(report)
	return nil
}
