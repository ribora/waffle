package outbox

import (
	"context"
	"testing"

	"github.com/ribora/hilton"
	"github.com/ribora/pry"
	"github.com/ribora/waffle/app"
)

func run(t *testing.T, store Store, args ...string) error {
	t.Helper()
	command := NewOutboxCommand(app.NewCommand("outbox", "test"), NewRelay(store, Recorder(), pry.Noop()), store)
	ctx := app.NewContextWithCommand("outbox", append([]string{"outbox"}, args...), hilton.Noop(), pry.Noop())
	return command.Run(ctx)
}

// The subcommand and its operand come from getopt, which has already dropped the
// command name — counting them out of the raw args again shifts both by one.
func TestResetTakesTheIdAfterTheSubcommand(t *testing.T) {
	store := &resetStore{}

	if err := run(t, store, "reset", "01a0-b49e"); err != nil {
		t.Fatalf("run: %v", err)
	}

	if store.id != "01a0-b49e" {
		t.Errorf("reset id = %q", store.id)
	}
}

func TestResetWithoutAnIdResetsEveryEntry(t *testing.T) {
	store := &resetStore{id: "unset"}

	if err := run(t, store, "reset"); err != nil {
		t.Fatalf("run: %v", err)
	}

	if store.id != "" {
		t.Errorf("reset id = %q, want the empty id that means all of them", store.id)
	}
}

func TestAnUnknownSubcommandIsRefused(t *testing.T) {
	if err := run(t, &resetStore{}, "drain"); err == nil {
		t.Fatal("an unknown subcommand was accepted")
	}
}

type resetStore struct {
	fakeStore
	id string
}

func (s *resetStore) Reset(_ context.Context, id string) (int, error) {
	s.id = id
	return 0, nil
}
