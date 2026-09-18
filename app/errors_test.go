package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"gitlab.com/gorib/failure"
	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/pry"
)

const commandDuration = "cli.command.duration"

var errNotFound = failure.New("resource not found")

func TestErrorType(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"success", nil, ""},
		{"usage", ErrUsage, "usage"},
		{"wrapped usage", fmt.Errorf("ctx: %w", ErrUsage), "usage"},
		{"failure carries its type", failure.Wrap("%w: id={id}", errNotFound, 7), "resource_not_found"},
		{"panic", failure.Panic("boom"), "panic"},
		{"plain error", errors.New("boom"), otherErrorType},
		{"wrapped plain error", fmt.Errorf("cannot connect to db: %w", errors.New("refused")), otherErrorType},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := errorType(c.err); got != c.want {
				t.Errorf("errorType(%v) = %q, want %q", c.err, got, c.want)
			}
		})
	}
}

func tracedErrorType(t *testing.T, run func(context.Context) error) any {
	t.Helper()

	rec := hilton.Recorder()
	a, err := Init([]Command{command("serve", run)}, pry.Noop(), rec)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	withArgv(t, "app", "serve")
	_ = a.Run()

	for _, m := range rec.Measurements() {
		if m.Instrument != commandDuration {
			continue
		}
		for _, attr := range m.Attributes {
			if attr.Key() == "error.type" {
				return attr.Value()
			}
		}
		return nil
	}
	t.Fatalf("%s was never recorded", commandDuration)
	return nil
}

// A key present on every success is a key nobody can group by.
func TestSuccessCarriesNoErrorType(t *testing.T) {
	if got := tracedErrorType(t, func(context.Context) error { return nil }); got != nil {
		t.Errorf("error.type = %v, want it absent", got)
	}
}

func TestCommandOutcomeReachesTheHistogram(t *testing.T) {
	cases := []struct {
		name string
		run  func(context.Context) error
		want any
	}{
		{"returned", func(context.Context) error { return errNotFound }, "resource_not_found"},
		{"panicked", func(context.Context) error { panic("boom") }, "panic"},
		{"usage", func(context.Context) error { return ErrUsage }, "usage"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := tracedErrorType(t, c.run); got != c.want {
				t.Errorf("error.type = %v, want %v", got, c.want)
			}
		})
	}
}
