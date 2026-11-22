package store

import (
	"context"
	"embed"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ribora/sql"
	"github.com/ribora/sql/sqltest"
	"github.com/ribora/waffle/tools/migrate"
	"github.com/ribora/waffle/tools/outbox"
	"github.com/ribora/waffle/tools/outbox/outboxtest"
)

// Embedded from a test file on purpose: the schema belongs to the service, which
// gets its own copy from `waffle make:outbox` and may edit it. This copy is only so
// the reference Store is tested against the table it was written for, and
// cmd/waffle keeps the two byte-identical.
//
//go:embed migrations/*.sql
var migrations embed.FS

// Without OUTBOX_TEST_DSN a container is started and the next run reuses it.
var postgres = sqltest.New(
	sqltest.Env("OUTBOX_TEST_DSN"),
	sqltest.Docker("postgres:18-alpine"),
	sqltest.Database("outbox_test"),
	sqltest.Ephemeral(),
	sqltest.Migrations(migrate.Up(migrations)),
)

var (
	provisioned sync.Once
	upErr       error
)

func reset(ctx context.Context) error {
	provisioned.Do(func() {
		if upErr = postgres.Provision(ctx); upErr == nil {
			upErr = postgres.Migrate(ctx)
		}
	})
	if upErr != nil {
		return upErr
	}
	return postgres.Reset(ctx)
}

func TestMain(m *testing.M) {
	code := m.Run()
	_ = postgres.Down(context.Background())
	os.Exit(code)
}

// The reference implementation has to pass the suite it ships.
func TestStoreConformance(t *testing.T) {
	outboxtest.Suite{
		Store: func(t *testing.T) outbox.Store {
			t.Helper()
			up(t)
			store, err := NewStore(postgres.Db())
			if err != nil {
				t.Fatalf("store: %v", err)
			}
			return store
		},
		Begin: func(ctx context.Context) (context.Context, func() error, func() error, error) {
			return sql.NewContextWithTransaction(ctx, postgres.Db())
		},
	}.Run(t)
}

// An entry past its attempts stops being claimed, is kept for diagnosis, and comes
// back when reset. WithMaxAttempts is a store option, so outboxtest cannot see it.
func TestExhaustedEntriesStopAndCanBeReset(t *testing.T) {
	up(t)
	ctx := context.Background()
	store, err := NewStore(postgres.Db(), WithMaxAttempts(1))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	entry := outbox.Entry{Id: "01950000-0000-7000-8000-000000000001", Subject: "poison", Payload: []byte("{}")}
	if err := store.Add(ctx, entry); err != nil {
		t.Fatalf("add: %v", err)
	}

	claimed, err := store.Claim(ctx, "relay", 0)
	if err != nil || claimed == nil {
		t.Fatalf("claim: %v, %v", claimed, err)
	}
	if err := store.Failed(ctx, claimed.Id, errors.New("nope")); err != nil {
		t.Fatalf("failed: %v", err)
	}

	again, err := store.Claim(ctx, "relay", 0)
	if err != nil {
		t.Fatalf("claim after exhaustion: %v", err)
	}
	if again != nil {
		t.Fatal("an entry past its attempts was claimed")
	}

	backlog, err := store.Backlog(ctx)
	if err != nil {
		t.Fatalf("backlog: %v", err)
	}
	if backlog.Unsent != 1 || backlog.Failed != 1 {
		t.Fatalf("backlog: got %d unsent and %d failed, want 1 and 1", backlog.Unsent, backlog.Failed)
	}

	reset, err := store.Reset(ctx, "")
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if reset != 1 {
		t.Fatalf("reset %d entries, want 1", reset)
	}
	// Reset drops the claim too, so it does not wait for one to go stale.
	retried, err := store.Claim(ctx, "relay", held)
	if err != nil || retried == nil {
		t.Fatalf("claim after reset: %v, %v", retried, err)
	}
	if retried.Attempts != 0 || retried.LastError != "" {
		t.Errorf("reset left %d attempts and %q", retried.Attempts, retried.LastError)
	}
}

// A limit that divides the rune width cuts cleanly, so the cases that matter are
// the ones where it does not: Postgres refuses invalid UTF-8 and takes the whole
// of Failed down with it.
func TestTruncateCutsOnARuneBoundary(t *testing.T) {
	for _, c := range []struct{ name, text string }{
		{"ascii", strings.Repeat("x", maxLastError*2)},
		{"two-byte runes", strings.Repeat("я", maxLastError)},
		{"three-byte runes", strings.Repeat("日", maxLastError)},
		{"four-byte runes", strings.Repeat("🙂", maxLastError)},
		{"one ascii, then two-byte runes", "x" + strings.Repeat("я", maxLastError)},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := truncate(c.text, maxLastError)
			if len(got) > maxLastError {
				t.Errorf("kept %d bytes, want at most %d", len(got), maxLastError)
			}
			if !utf8.ValidString(got) {
				t.Errorf("cut mid-rune: %q", got[len(got)-4:])
			}
		})
	}
}

const held = time.Hour

func up(t *testing.T) {
	t.Helper()
	if err := reset(context.Background()); err != nil {
		t.Skipf("no postgres for the outbox store: %v", err)
	}
}
