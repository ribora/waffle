package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"

	"golang.org/x/exp/maps"

	"github.com/pborman/getopt/v2"
)

type commandTag struct{}

var CommandTag = commandTag{}

type BaseCommand interface {
	Name() string
	Description() string
	Usage() error
}

type Command interface {
	BaseCommand
	Run(ctx context.Context) error
}

type CommandList map[string]Command

func (l *CommandList) Add(command Command) error {
	if command.Name() == "" {
		return errors.New("empty command name")
	}
	if *l == nil {
		*l = make(CommandList)
	}
	if _, ok := (*l)[command.Name()]; ok {
		return fmt.Errorf("command '%s' has been already registered", command.Name())
	}
	(*l)[command.Name()] = command
	return nil
}

func (l *CommandList) Get(command string) Command {
	return (*l)[command]
}

func (l *CommandList) List() <-chan Command {
	ch := make(chan Command, len(*l))
	commands := maps.Values(*l)
	sort.SliceStable(
		commands,
		func(i, j int) bool {
			return commands[i].Name() < commands[j].Name()
		},
	)
	for _, value := range commands {
		ch <- value
	}
	close(ch)
	return ch
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

func (b *baseCommand) Usage() error {
	var help bool
	getopt.FlagLong(&help, "help", 'h', "Display this help message")
	getopt.Parse()
	if help {
		getopt.SetParameters("")
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", b.Description())
		getopt.PrintUsage(os.Stderr)
		return ErrUsage
	}
	return nil
}
