package converter

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

// converter command has no per-command templates of its own — the shared
// templates live next to generate.go in this same package.
var noTemplates embed.FS

func New() *cmd {
	return &cmd{
		base: command.New("make:converter",
			"Scaffold a goverter dto/convert.go at the given path (creates pkg/goverter/convert.go too if missing)",
			noTemplates, nil),
	}
}

type cmd struct {
	base *command.Command
}

func (c *cmd) Code() string        { return c.base.Code() }
func (c *cmd) Description() string { return c.base.Description() }

func (c *cmd) Run(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: waffle %s <path>\n  e.g. waffle %s domain/users/cmd/grpc/convert\n       waffle %s domain/orders/adapter/external-api/dto",
			c.Code(), c.Code(), c.Code())
	}
	targetDir := strings.TrimRight(args[0], string(filepath.Separator))

	projectName, err := c.base.FindProjectName()
	if err != nil {
		return err
	}

	if err := Generate(targetDir, projectName); err != nil {
		return err
	}

	fmt.Printf("Scaffolded %s/convert.go.\n", targetDir)
	fmt.Println("Edit the file to declare conversion methods, then run `make generate`.")
	return nil
}
