package app

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/fatih/color"
	"github.com/pborman/getopt/v2"
	"github.com/sirupsen/logrus"

	"gitlab.com/gorib/try"
)

type Configure interface{}

type App interface {
	RunUntilStop()
	Run()
	Stop()
}

func Init(commands []Command) *app {
	var cmd CommandList
	for _, command := range commands {
		cmd.Add(command)
	}
	return &app{commands: cmd}
}

type app struct {
	commands CommandList
	config   Configure
	closer   func()
}

func (a app) RunUntilStop() {
	go a.Watch()
	a.Run()
}

func (a app) Run() {
	opts := getopt.New()
	var help bool
	opts.FlagLong(&help, "help", 'h', "Display this help message")
	opts.SetParameters("command")

	opts.Parse(os.Args)
	command := opts.Arg(0)
	if command == "" {
		help = true
	}
	if help {
		a.usage(opts.PrintUsage)
	}

	if cmd := a.commands.Get(command); cmd != nil {
		ctx := NewContextWithCommand(cmd.Name(), opts.Args())
		os.Args = opts.Args()
		try.Catch(func() {
			cmd.Run(ctx)
		}, func(throwable error) {
			logrus.WithContext(ctx).Error(throwable)
			os.Exit(1)
		})
	} else {
		color.Red("Unknown command '%s'.\n", command)
		a.usage(opts.PrintUsage)
		os.Exit(2)
	}
}

func (a app) Stop() {
	if a.closer != nil {
		a.closer()
	}
}

func (a app) Watch() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	a.Stop()
}

func (a app) usage(printer func(w io.Writer)) {
	printer(os.Stderr)
	_, _ = fmt.Fprintf(os.Stderr, "Available commands:\n")
	var names []string
	for command := range a.commands {
		names = append(names, command)
	}
	sort.Strings(names)
	for cmd := range a.commands.List() {
		fmt.Printf("\t%s\t%s\n", color.New(color.FgGreen).Sprint(cmd.Name()), cmd.Description())
	}
	os.Exit(2)
}
