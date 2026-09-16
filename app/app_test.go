package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/hilton/otel"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/session"
)

type stub struct {
	*baseCommand
	run  func(ctx context.Context) error
	seen context.Context
}

func (s *stub) Run(ctx context.Context) error {
	s.seen = ctx
	if s.run == nil {
		return nil
	}
	return s.run(ctx)
}

func command(name string, run func(ctx context.Context) error) *stub {
	return &stub{baseCommand: NewCommand(name, name+" description"), run: run}
}

type awareStub struct {
	*stub
	got Commands
}

func (a *awareStub) SetCommands(c Commands) { a.got = c }

func logger(*testing.T) pry.Logger {
	return pry.Noop()
}

func application(t *testing.T, commands ...Command) *app {
	t.Helper()
	a, err := Init(commands, logger(t), hilton.Noop())
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	return a
}

// Run reads the real os.Args, so a test has to own them for its duration.
func withArgv(t *testing.T, argv ...string) {
	t.Helper()
	saved := os.Args
	t.Cleanup(func() { os.Args = saved })
	os.Args = argv
}

func TestRegistrationRejectsDuplicatesAndBlanks(t *testing.T) {
	if _, err := Init([]Command{command("", nil)}, logger(t), hilton.Noop()); err == nil {
		t.Error("a command with no name is unreachable from the CLI, so it must not register")
	}
	_, err := Init([]Command{command("serve", nil), command("serve", nil)}, logger(t), hilton.Noop())
	if err == nil {
		t.Error("the second registration silently won the map")
	}
}

// A command that dispatches to others (cron) needs the list before it runs.
func TestCommandsAwareIsWiredWithTheFullList(t *testing.T) {
	aware := &awareStub{stub: command("cron", nil)}
	late := command("serve", nil)
	application(t, aware, late)

	if aware.got == nil {
		t.Fatal("SetCommands was never called")
	}
	if aware.got.Get("serve") != late {
		t.Error("the list was handed over before every command was in it")
	}
}

// The exit code is the only thing a shell script can read.
func TestExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"success", nil, 0},
		{"usage", ErrUsage, 2},
		{"wrapped usage", errors.Join(errors.New("context"), ErrUsage), 2},
		{"failure", errors.New("boom"), 1},
	}
	for _, c := range cases {
		if got := exitCode(c.err); got != c.want {
			t.Errorf("%s = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestRunRejectsWhatItCannotDispatch(t *testing.T) {
	cases := map[string][]string{
		"no command":      {"app"},
		"unknown command": {"app", "nope"},
		"help":            {"app", "--help"},
	}
	for name, argv := range cases {
		t.Run(name, func(t *testing.T) {
			ran := false
			a := application(t, command("serve", func(context.Context) error { ran = true; return nil }))
			withArgv(t, argv...)

			if err := a.Run(); !errors.Is(err, ErrUsage) {
				t.Errorf("err = %v, want ErrUsage", err)
			}
			if ran {
				t.Error("a command ran anyway")
			}
		})
	}
}

func TestRunReturnsTheCommandsError(t *testing.T) {
	want := errors.New("boom")
	a := application(t, command("serve", func(context.Context) error { return want }))
	withArgv(t, "app", "serve")

	if err := a.Run(); !errors.Is(err, want) {
		t.Errorf("err = %v, want the command's own error", err)
	}
}

// A panic in one command must not take down a process that is mid-shutdown.
func TestRunTurnsAPanicIntoAnError(t *testing.T) {
	a := application(t, command("serve", func(context.Context) error { panic("boom") }))
	withArgv(t, "app", "serve")

	err := a.Run()
	if err == nil {
		t.Fatal("the panic escaped as a crash")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v, want the panic value", err)
	}
}

// Everything a command needs arrives through the context and nowhere else.
func TestTheCommandContextIsFurnished(t *testing.T) {
	log := logger(t)
	cmd := command("serve", nil)
	a, err := Init([]Command{cmd}, log, hilton.Noop())
	if err != nil {
		t.Fatal(err)
	}
	withArgv(t, "app", "serve", "--flag", "value")

	if err := a.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	ctx := cmd.seen
	if ctx == nil {
		t.Fatal("the command never ran")
	}

	// The command name leads, because (*getopt.Set).Getopt reads args[0] as the
	// program name and would otherwise swallow the first real flag.
	want := []string{"serve", "--flag", "value"}
	if got := Args(ctx); !slices.Equal(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
	if pry.LoggerFromContext(ctx) != log {
		t.Error("the command got a different logger than the app was built with")
	}
	s := session.FromContext(ctx)
	if s == nil {
		t.Fatal("no session in the context")
	}
	if s.Uri != "serve" {
		t.Errorf("session uri = %q, want the command name", s.Uri)
	}
	if s.Method != "CLI" {
		t.Errorf("session method = %q", s.Method)
	}
	if s.Body != strings.Join(want, " ") {
		t.Errorf("session body = %q, want the invocation", s.Body)
	}
}

// ProviderFromContext falls back to a noop, so only a recording provider tells
// "wired" apart from "absent".
func TestTheSpanReachesTheSession(t *testing.T) {
	provider, err := otel.NewProvider("waffle-test", "test", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cmd := command("serve", nil)
	a, err := Init([]Command{cmd}, logger(t), provider)
	if err != nil {
		t.Fatal(err)
	}
	withArgv(t, "app", "serve")

	if err := a.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if hilton.ProviderFromContext(cmd.seen) != provider {
		t.Error("the command cannot start its own spans on the app's provider")
	}
	s := session.FromContext(cmd.seen)
	if s == nil {
		t.Fatal("no session in the context")
	}
	if s.Traceparent == "" {
		t.Error("no traceparent, so a downstream call starts a trace of its own")
	}
}

func TestStopWithoutACloser(t *testing.T) {
	if err := application(t).Stop(); err != nil {
		t.Errorf("err = %v, want nil for an app that registered no closer", err)
	}
}

// Both exits have to reach the closer, and a failing closer has to be reported.
func TestWatch(t *testing.T) {
	t.Run("context cancel", func(t *testing.T) {
		a := application(t)
		stopped := make(chan struct{})
		a.SetCloser(func() error { close(stopped); return nil })

		ctx, cancel := context.WithCancel(context.Background())
		closed := make(chan error, 1)
		go a.Watch(ctx, closed)
		cancel()

		if err := <-closed; err != nil {
			t.Errorf("err = %v", err)
		}
		select {
		case <-stopped:
		default:
			t.Error("the closer never ran")
		}
	})

	t.Run("closer error", func(t *testing.T) {
		want := errors.New("connection still draining")
		a := application(t)
		a.SetCloser(func() error { return want })

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		closed := make(chan error, 1)
		go a.Watch(ctx, closed)

		if err := <-closed; !errors.Is(err, want) {
			t.Errorf("err = %v, want the closer's error", err)
		}
	})

	t.Run("signal", func(t *testing.T) {
		// Registered first so the signal is never unhandled: until Watch calls
		// signal.Notify, the default action would kill the test binary.
		guard := make(chan os.Signal, 1)
		signal.Notify(guard, syscall.SIGTERM)
		defer signal.Stop(guard)

		a := application(t)
		stopped := make(chan struct{})
		a.SetCloser(func() error { close(stopped); return nil })

		closed := make(chan error, 1)
		go a.Watch(context.Background(), closed)

		// Watch's own Notify may not be in place yet, and a signal delivered
		// before it is simply lost.
		deadline := time.After(5 * time.Second)
		for {
			if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
				t.Fatalf("kill: %v", err)
			}
			select {
			case <-stopped:
				if err := <-closed; err != nil {
					t.Errorf("err = %v", err)
				}
				return
			case <-deadline:
				t.Fatal("SIGTERM never reached the closer")
			case <-time.After(20 * time.Millisecond):
			}
		}
	})
}

// RunUntilStop ends the process, so it can only be observed from outside: the
// exit code is the contract, and the closer has to have run before it.
const (
	childEnv    = "WAFFLE_APP_CHILD"
	markerEnv   = "WAFFLE_APP_MARKER"
	childRunner = "-test.run=TestRunUntilStopExitCodes"
)

func TestRunUntilStopExitCodes(t *testing.T) {
	if mode := os.Getenv(childEnv); mode != "" {
		runChild(mode)
		return
	}

	cases := []struct {
		mode string
		want int
	}{
		{"success", 0},
		{"usage", 2},
		{"failure", 1},
	}
	for _, c := range cases {
		t.Run(c.mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "closed")
			child := exec.Command(os.Args[0], childRunner)
			child.Env = append(os.Environ(), childEnv+"="+c.mode, markerEnv+"="+marker)

			got := 0
			var exit *exec.ExitError
			switch err := child.Run(); {
			case errors.As(err, &exit):
				got = exit.ExitCode()
			case err != nil:
				t.Fatalf("child: %v", err)
			}
			if got != c.want {
				t.Errorf("exit = %d, want %d", got, c.want)
			}
			if _, err := os.Stat(marker); err != nil {
				t.Error("the process exited without running the closer")
			}
		})
	}
}

func runChild(mode string) {
	a, err := Init([]Command{command("serve", func(context.Context) error {
		if mode == "failure" {
			return errors.New("boom")
		}
		return nil
	})}, pry.Noop(), hilton.Noop())
	if err != nil {
		panic(err)
	}
	marker := os.Getenv(markerEnv)
	a.SetCloser(func() error { return os.WriteFile(marker, nil, 0o644) })

	// No command at all is how a user gets the usage exit code.
	os.Args = []string{"app", "serve"}
	if mode == "usage" {
		os.Args = []string{"app"}
	}
	a.RunUntilStop()
}
