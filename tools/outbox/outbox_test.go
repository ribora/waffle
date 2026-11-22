package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ribora/session"
)

func TestPublisher(t *testing.T) {
	// Publish goes straight to the sender, so a rollback after it cannot recall
	// the event.
	t.Run("PublishSendsNow", func(t *testing.T) {
		store := &fakeStore{}
		sender := Recorder()
		publisher := NewPublisher(store, sender)

		if err := publisher.Publish(context.Background(), "went.out", map[string]int{"a": 1}); err != nil {
			t.Fatalf("publish: %v", err)
		}
		if got := len(sender.Sent()); got != 1 {
			t.Fatalf("sent %d messages, want 1", got)
		}
		if len(store.added) != 0 {
			t.Fatalf("publish wrote %d rows, want none", len(store.added))
		}
	})

	t.Run("EnqueueOnlyWrites", func(t *testing.T) {
		store := &fakeStore{}
		sender := Recorder()
		publisher := NewPublisher(store, sender)

		if err := publisher.Enqueue(context.Background(), "invoices.paid", map[string]int{"id": 7}); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		if got := len(sender.Sent()); got != 0 {
			t.Fatalf("enqueue sent %d messages, want none", got)
		}
		if len(store.added) != 1 {
			t.Fatalf("enqueue wrote %d rows, want 1", len(store.added))
		}
		entry := store.added[0]
		if entry.Subject != "invoices.paid" {
			t.Errorf("subject: got %q", entry.Subject)
		}
		if string(entry.Payload) != `{"id":7}` {
			t.Errorf("payload: got %q", entry.Payload)
		}
		if entry.Id == "" {
			t.Error("no id was generated")
		}
	})

	t.Run("EnqueueKeepsTheSession", func(t *testing.T) {
		store := &fakeStore{}
		publisher := NewPublisher(store, Recorder())

		ctx := session.NewContextWithSession(context.Background(), session.Session{CorrelationId: "corr-9"})
		ctx = session.WithTrace(ctx, "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01", "vendor=1")
		if err := publisher.Enqueue(ctx, "k", "payload"); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		entry := store.added[0]
		if entry.CorrelationId != "corr-9" {
			t.Errorf("correlation id: got %q", entry.CorrelationId)
		}
		if entry.Traceparent == "" {
			t.Error("no traceparent was stored")
		}
		if entry.Tracestate != "vendor=1" {
			t.Errorf("tracestate: got %q", entry.Tracestate)
		}
	})

	t.Run("EnqueueReportsAnEncodingFailure", func(t *testing.T) {
		store := &fakeStore{}
		publisher := NewPublisher(store, Recorder())

		if err := publisher.Enqueue(context.Background(), "k", func() {}); err == nil {
			t.Fatal("a payload that cannot be encoded was accepted")
		}
		if len(store.added) != 0 {
			t.Fatalf("a row was written for an unencodable payload")
		}
	})
}

// Enough of a Store for the publisher and the relay, and unexported on purpose: a
// memory store cannot honour the requirement the real ones are judged on, and
// shipping one would invite somebody to run on it.
type fakeStore struct {
	added    []Entry
	claim    *Entry
	claimed  []string
	sent     []string
	sentErr  error
	failed   []string
	causes   []error
	claimErr error
	addErr   error

	sentDeadline bool
	sentLeft     time.Duration
}

func (f *fakeStore) Add(_ context.Context, entry Entry) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.added = append(f.added, entry)
	return nil
}

func (f *fakeStore) Claim(_ context.Context, by string, _ time.Duration) (*Entry, error) {
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	entry := f.claim
	f.claim = nil
	if entry != nil {
		f.claimed = append(f.claimed, by)
	}
	return entry, nil
}

// Read here and not kept: the relay cancels the context it hands over as soon as
// the call returns, so a test holding one would inspect it after the fact.
func (f *fakeStore) Sent(ctx context.Context, id string) error {
	f.sentErr = ctx.Err()
	if deadline, ok := ctx.Deadline(); ok {
		f.sentDeadline, f.sentLeft = true, time.Until(deadline)
	}
	f.sent = append(f.sent, id)
	return nil
}

func (f *fakeStore) Failed(_ context.Context, id string, cause error) error {
	f.failed = append(f.failed, id)
	f.causes = append(f.causes, cause)
	return nil
}

func (f *fakeStore) Purge(context.Context, time.Duration) (int, error) { return 0, nil }

func (f *fakeStore) Backlog(context.Context) (Backlog, error) { return Backlog{}, nil }

func (f *fakeStore) Reset(context.Context, string) (int, error) { return 0, nil }

var errRefused = errors.New("broker refused")
