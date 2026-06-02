package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/pborman/getopt/v2"
)

var ItemColor = color.New(color.FgGreen)

type commandTag struct{}

var CommandTag = commandTag{}

type BaseCommand interface {
	Name() string
	Description() string
	Usage(ctx context.Context) error
}

type Command interface {
	BaseCommand
	Run(ctx context.Context) error
}

type Daemon interface {
	Command
	Daemon()
}

type Commands interface {
	Get(name string) Command
}

type CommandsAware interface {
	SetCommands(Commands)
}

type commandList map[string]Command

func (l commandList) Add(command Command) error {
	if command.Name() == "" {
		return errors.New("empty command name")
	}
	if _, ok := l[command.Name()]; ok {
		return fmt.Errorf("command '%s' has been already registered", command.Name())
	}
	l[command.Name()] = command
	return nil
}

func (l commandList) Get(command string) Command {
	return l[command]
}

func PrintList(w io.Writer, header string, items map[string]string) {
	if len(items) == 0 {
		return
	}
	names := make([]string, 0, len(items))
	width := 0
	for n := range items {
		names = append(names, n)
		width = max(width, len(n))
	}
	sort.Strings(names)
	_, _ = fmt.Fprintln(w, header)
	for _, n := range names {
		name := ItemColor.Sprint(n) + strings.Repeat(" ", width+1-len(n))
		_, _ = fmt.Fprintf(w, "\t%s\t%s\n", name, items[n])
	}
}

func NewCommand(name, description string) *baseCommand {
	return &baseCommand{
		name:        name,
		description: description,
	}
}

type baseCommand struct {
	name, description string
}

func (b *baseCommand) Name() string {
	return b.name
}

func (b *baseCommand) Description() string {
	return b.description
}

func (b *baseCommand) Usage(ctx context.Context) error {
	opts := getopt.New()
	var help bool
	opts.FlagLong(&help, "help", 'h', "Display this help message")
	if err := opts.Getopt(Args(ctx), nil); err != nil {
		return err
	}
	if help {
		opts.SetParameters("")
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", b.Description())
		opts.PrintUsage(os.Stderr)
		return ErrUsage
	}
	return nil
}
