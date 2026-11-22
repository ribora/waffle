package converter

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command"
)

func New() *cmd {
	return &cmd{
		base: command.New("make:converter",
			"Scaffold a goverter dto/convert.go at the given path (creates pkg/goverter/convert.go too if missing)").
			WithUsage("<path>\n  e.g. waffle make:converter domain/users/cmd/grpc/convert\n" +
				"       waffle make:converter domain/orders/adapter/external-api/dto"),
	}
}

type cmd struct {
	base *command.Command
}

func (c *cmd) Code() string        { return c.base.Code() }
func (c *cmd) Description() string { return c.base.Description() }
func (c *cmd) Usage() string       { return c.base.Usage() }

func (c *cmd) Run(ctx context.Context, args ...string) error {
	if err := c.base.Operands(args, 1, 1); err != nil {
		return err
	}
	// A path, not an identifier, so the path rule applies instead of ValidateName.
	if err := command.ValidatePath("path", args[0]); err != nil {
		return err
	}
	targetDir := strings.TrimRight(args[0], string(filepath.Separator))

	p, err := command.OpenProject()
	if err != nil {
		return err
	}
	if err := Generate(p, targetDir); err != nil {
		return err
	}

	fmt.Printf("Scaffolded %s/convert.go.\n", targetDir)
	fmt.Println("Edit the file to declare conversion methods, then run `make generate`.")
	return nil
}
