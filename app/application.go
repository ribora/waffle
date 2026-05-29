package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/pborman/getopt/v2"

	"gitlab.com/gorib/failure"
	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/session"
)

type Configure interface{}

type App interface {
	RunUntilStop()
	Run() int
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
	ctx, cancel := context.WithCancel(context.Background())

	closed := make(chan error, 1)
	go a.Watch(ctx, closed)
	code := a.Run()
	cancel()
	if err := <-closed; err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to gracefully shutdown an application: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func (a *app) Run() int {
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
		return 2
	}

	if cmd := a.commands.Get(command); cmd != nil {
		ctx := NewContextWithCommand(cmd.Name(), opts.Args(), a.provider, a.logger)
		os.Args = opts.Args()

		const instrumentationName = "gitlab.com/gorib/waffle/app"
		tracer := a.provider.Tracer(instrumentationName)
		meter := a.provider.Meter(instrumentationName)

		duration := meter.Histogram("cli.command.duration", hilton.WithDescription("Duration of CLI commands"), hilton.WithUnit("s"))

		ctx, span := tracer.Start(ctx, "cmd."+cmd.Name(),
			hilton.WithSpanKind(hilton.SpanKindInternal),
			hilton.WithAttributes(hilton.StringAttr("cli.command", cmd.Name())),
		)
		defer span.End()

		if s := session.FromContext(ctx); s != nil {
			if tp := hilton.FormatTraceparent(span.SpanContext()); tp != "" {
				s.Traceparent = tp
			}
		}

		start := time.Now()
		err := failure.Catch(func() error { return cmd.Run(ctx) })
		took := time.Since(start).Seconds()

		status := "ok"
		if err != nil {
			status = "error"
			span.RecordError(err)
			span.SetStatus(hilton.StatusError, err.Error())
		}

		attrs := []hilton.Attribute{
			hilton.StringAttr("cli.command", cmd.Name()),
			hilton.StringAttr("status", status),
		}
		duration.Record(ctx, took, attrs...)

		if err != nil {
			a.logger.Error(err, pry.Ctx(ctx))
			return 1
		}
	} else {
		color.Red("Unknown command '%s'.\n", command)
		a.usage(opts.PrintUsage)
		return 2
	}
	return 0
}

func (a *app) Stop() error {
	if a.closer != nil {
		return a.closer()
	}
	return nil
}

func (a *app) Watch(ctx context.Context, closed chan error) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
	case <-ctx.Done():
	}
	if err := a.Stop(); err != nil {
		closed <- err
	}
	close(closed)
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
}
