package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/pborman/getopt/v2"

	"gitlab.com/gorib/failure"
	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/session"
)

const instrumentationName = "gitlab.com/gorib/waffle/app"

type Configure interface{}

type App interface {
	RunUntilStop()
	Run() error
	Stop() error
}

type HasCloser interface {
	SetCloser(func() error)
	Closer() func() error
}

func Init(commands []Command, logger pry.Logger, provider hilton.Provider) (*app, error) {
	cmd := make(commandList, len(commands))
	for _, command := range commands {
		if err := cmd.Add(command); err != nil {
			return nil, err
		}
	}
	for _, command := range commands {
		if a, ok := command.(CommandsAware); ok {
			a.SetCommands(cmd)
		}
	}
	return &app{commands: cmd, logger: logger, provider: provider}, nil
}

type app struct {
	commands commandList
	closer   func() error
	logger   pry.Logger
	provider hilton.Provider
}

func (a *app) RunUntilStop() {
	ctx, cancel := context.WithCancel(context.Background())

	closed := make(chan error, 1)
	go a.Watch(ctx, closed)
	runErr := a.Run()
	cancel()

	if err := <-closed; err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to gracefully shutdown an application: %v\n", err)
	}

	os.Exit(exitCode(runErr))
}

// exitCode maps a Run() error to a POSIX-style process exit code.
//
//	nil       → 0  success
//	ErrUsage  → 2  user invoked the command incorrectly
//	other     → 1  runtime failure
func exitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ErrUsage):
		return 2
	default:
		return 1
	}
}

func (a *app) Run() error {
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
		return ErrUsage
	}

	cmd := a.commands.Get(command)
	if cmd == nil {
		_, _ = color.New(color.FgRed).Fprintf(os.Stderr, "Unknown command '%s'.\n", command)
		a.usage(opts.PrintUsage)
		return ErrUsage
	}

	ctx := NewContextWithCommand(cmd.Name(), opts.Args(), a.provider, a.logger)

	tracer := a.provider.Tracer(instrumentationName)
	meter := a.provider.Meter(instrumentationName)

	// Not WithSecondsBoundaries: commands run for minutes, not milliseconds.
	duration := meter.Histogram("cli.command.duration",
		hilton.WithDescription("Duration of CLI commands"),
		hilton.WithUnit("s"),
		hilton.WithBoundaries(0.1, 0.5, 1, 5, 10, 30, 60, 300, 900),
	)

	ctx, span := tracer.Start(ctx, "cmd."+cmd.Name(),
		hilton.WithSpanKind(hilton.SpanKindInternal),
		hilton.WithAttributes(hilton.Attr("cli.command", cmd.Name())),
	)
	defer span.End()

	if tp := hilton.FormatTraceparent(span.SpanContext()); tp != "" {
		ctx = session.WithTraceparent(ctx, tp)
	}

	start := time.Now()
	err := failure.Catch(func() error { return cmd.Run(ctx) })
	took := time.Since(start).Seconds()

	attrs := []hilton.Attribute{
		hilton.Attr("cli.command", cmd.Name()),
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(hilton.StatusError, err.Error())
		attrs = append(attrs, hilton.Attr("error.type", errorType(err)))
	}
	duration.Record(ctx, took, attrs...)

	if err != nil && !errors.Is(err, ErrUsage) {
		a.logger.Error(err, pry.Ctx(ctx))
	}
	return err
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
	defer signal.Stop(sig)
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
	items := make(map[string]string, len(a.commands))
	for name, cmd := range a.commands {
		items[name] = cmd.Description()
	}
	PrintList(os.Stderr, "Available commands:", items)
}
