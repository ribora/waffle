package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ribora/hilton"
)

func TestArgsOnAContextThatHasNone(t *testing.T) {
	if got := Args(context.Background()); got != nil {
		t.Errorf("args = %v, want nil", got)
	}
}

// cron re-invokes another command with an argv the process never got.
func TestWithArgsReplacesTheList(t *testing.T) {
	base := NewContextWithCommand("cron", []string{"cron"}, hilton.Noop(), logger(t))
	want := []string{"serve", "--once"}
	if got := Args(WithArgs(base, want)); !slices.Equal(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

func TestPrintList(t *testing.T) {
	var empty strings.Builder
	PrintList(&empty, "Available commands:", nil)
	if empty.Len() != 0 {
		t.Errorf("an empty list still printed its header: %q", empty.String())
	}

	var out strings.Builder
	PrintList(&out, "Available commands:", map[string]string{"serve": "run it", "db": "migrate"})
	body := out.String()
	if !strings.HasPrefix(body, "Available commands:\n") {
		t.Errorf("no header:\n%s", body)
	}
	// Sorted, so the same set of commands always prints the same help.
	if strings.Index(body, "db") > strings.Index(body, "serve") {
		t.Errorf("not sorted:\n%s", body)
	}
}

// Every command delegates -h here, so it has to stop the run, not just print.
func TestBaseCommandUsage(t *testing.T) {
	cmd := NewCommand("serve", "run it")
	ctx := func(args ...string) context.Context {
		return NewContextWithCommand("serve", args, hilton.Noop(), logger(t))
	}

	if err := cmd.Usage(ctx("serve")); err != nil {
		t.Errorf("err = %v, want nil when no help was asked for", err)
	}
	if err := cmd.Usage(ctx("serve", "--help")); !errors.Is(err, ErrUsage) {
		t.Errorf("err = %v, want ErrUsage", err)
	}
	if err := cmd.Usage(ctx("serve", "--nonsense")); err == nil {
		t.Error("an unknown flag was accepted")
	}
}
