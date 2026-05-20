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

	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/try"
)

type Configure interface{}

type App interface {
	RunUntilStop()
	Run()
	Stop() error
}

type HasCloser interface {
	SetCloser(func() error)
	Closer() func() error
}

func Init(commands []Command, logger pry.Logger, provider hilton.Provider) (*app, error) {
	var cmd CommandList
	for _, command := range commands {
		if err := cmd.Add(command); err != nil {
			return nil, err
		}
	}
	return &app{commands: cmd, logger: logger, provider: provider}, nil
}

type app struct {
	commands CommandList
	closer   func() error
	logger   pry.Logger
	provider hilton.Provider
}

func (a *app) RunUntilStop() {
	go a.Watch()
	a.Run()
}

func (a *app) Run() {
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
		ctx := NewContextWithCommand(cmd.Name(), opts.Args(), a.provider)
		os.Args = opts.Args()
		var err error
		try.Catch(func() {
			err = cmd.Run(ctx)
		}, func(throwable error) {
			err = throwable
		})
		if err != nil {
			a.logger.Error(err, pry.Ctx(ctx))
			os.Exit(1)
		}
	} else {
		color.Red("Unknown command '%s'.\n", command)
		a.usage(opts.PrintUsage)
		os.Exit(2)
	}
}

func (a *app) Stop() error {
	if a.closer != nil {
		return a.closer()
	}
	return nil
}

func (a *app) Watch() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	if err := a.Stop(); err != nil {
		panic(err)
	}
}

func (a *app) SetCloser(closer func() error) {
	a.closer = closer
}

func (a *app) Closer() func() error {
	return a.closer
}

func (a *app) usage(printer func(w io.Writer)) {
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
