// Package whisk wires the whisk generator into a project. Shared, because every
// command that scaffolds a layer with whisk-tagged DTOs needs the tool in place
// before the first `//go:generate go tool whisk` the author writes.
package whisk

import (
	"fmt"
	"os"
	"os/exec"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

const module = "gitlab.com/gorib/whisk/cmd/whisk"

func Ensure(c *command.Command) error {
	if err := c.EnsureTool(module); err != nil {
		return err
	}
	// whisk writes whisk.yaml itself rather than waffle carrying a second copy of
	// its defaults; it leaves an existing file alone and reports which happened.
	initCmd := exec.Command("go", "tool", "whisk", "init")
	initCmd.Stderr = os.Stderr
	if err := initCmd.Run(); err != nil {
		return fmt.Errorf("whisk init: %w", err)
	}
	return nil
}
