package main

import (
	"context"
	"fmt"
	"iter"
	"os"
	"slices"
	"strings"

	"github.com/fatih/color"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/agents"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/cmdcommand"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/converter"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/domain"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/grpc"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/http"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/migrate"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/migration"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/repository"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/scribus"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/service"
)

type Command interface {
	Code() string
	Description() string
	Run(ctx context.Context, args ...string) error
}

type commands struct {
	cmds map[string]Command
}

func (c *commands) Add(cmd Command) {
	if c.cmds == nil {
		c.cmds = make(map[string]Command)
	}
	c.cmds[cmd.Code()] = cmd
}

func (c *commands) Get(cmd string) Command {
	if c.cmds == nil {
		return nil
	}
	return c.cmds[cmd]
}

func (c *commands) All() iter.Seq2[string, Command] {
	return func(yield func(string, Command) bool) {
		var codes []string
		for code := range c.cmds {
			codes = append(codes, code)
		}
		slices.Sort(codes)
		for _, code := range codes {
			if !yield(code, c.cmds[code]) {
				return
			}
		}
	}
}

func main() {
	var cmds commands
	cmds.Add(service.New())
	cmds.Add(domain.New())
	cmds.Add(grpc.New())
	cmds.Add(http.New())
	cmds.Add(scribus.New())
	cmds.Add(migrate.New())
	cmds.Add(migration.New())
	cmds.Add(repository.New())
	cmds.Add(cmdcommand.New())
	cmds.Add(converter.New())
	cmds.Add(agents.New())

	if len(os.Args) < 2 {
		usage(cmds)
		os.Exit(2)
	}
	cmd := os.Args[1]
	command := cmds.Get(cmd)
	if command == nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unknown command %q\n", cmd)
		usage(cmds)
		os.Exit(1)
	}

	err := command.Run(context.Background(), os.Args[2:]...)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

var itemColor = color.New(color.FgGreen)

func usage(cmds commands) {
	width := 0
	for code := range cmds.cmds {
		if len(code) > width {
			width = len(code)
		}
	}
	_, _ = fmt.Fprintln(os.Stderr, "Available commands:")
	for code, command := range cmds.All() {
		name := itemColor.Sprint(code) + strings.Repeat(" ", width+1-len(code))
		_, _ = fmt.Fprintf(os.Stderr, "\t%s\t%s\n", name, command.Description())
	}
}
