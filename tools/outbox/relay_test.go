package outbox

import (
	"context"
	"testing"
	"time"

	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/pry"
)

const parentTrace = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

func TestRelay(t *testing.T) {
	t.Run("DeliversAndMarksSent", func(t *testing.T) {
		store := &fakeStore{claim: &Entry{Id: "e1", Subject: "invoices.paid", Payload: []byte(`{"id":7}`)}}
		sender := Recorder()
		relay := NewRelay(store, sender, pry.Recorder(), WithName("relay-1"))

		if !relay.pass(context.Background()) {
			t.Fatal("a pass with an entry in the store reported no work")
		}
		sent := sender.Sent()
		if len(sent) != 1 {
			t.Fatalf("sent %d messages, want 1", len(sent))
		}
		if sent[0].Subject != "invoices.paid" {
			t.Errorf("subject: got %q", sent[0].Subject)
		}
		if len(store.sent) != 1 || store.sent[0] != "e1" {
			t.Errorf("marked sent: %v", store.sent)
		}
		if len(store.claimed) != 1 || store.claimed[0] != "relay-1" {
			t.Errorf("claimed by: %v", store.claimed)
		}
	})

	// Whatever cancelled the caller's context, the delivery had already happened —
	// leaving it unmarked is what sends it a second time.
	t.Run("MarksSentAfterTheContextIsCancelled", func(t *testing.T) {
		store := &fakeStore{claim: &Entry{Id: "e1", Subject: "invoices.paid"}}
		relay := NewRelay(store, Recorder(), pry.Recorder())

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		relay.deliver(ctx)

		if len(store.sent) != 1 {
			t.Fatalf("marked sent: %v", store.sent)
		}
		if store.sentErr != nil {
			t.Errorf("the store was handed a cancelled context: %v", store.sentErr)
		}
		if !store.sentDeadline {
			t.Error("the store was handed a context that can never time out")
		}
	})

	// A slow broker must not spend the budget meant for recording what it did: the
	// entry is delivered by then, and a bookkeeping write that times out sends it twice.
	t.Run("TheBookkeepingDeadlineStartsAfterTheSend", func(t *testing.T) {
		store := &fakeStore{claim: &Entry{Id: "e1", Subject: "invoices.paid"}}
		relay := NewRelay(store, slowSender{take: 200 * time.Millisecond}, pry.Recorder())

		relay.deliver(context.Background())

		if len(store.sent) != 1 {
			t.Fatalf("marked sent: %v", store.sent)
		}
		if spent := bookkeepingTimeout - store.sentLeft; spent > 100*time.Millisecond {
			t.Errorf("the send spent %v of the bookkeeping budget", spent.Round(time.Millisecond))
		}
	})

	// OUTBOX_STALE_AFTER=0 reaches this the same way any other value does, and a
	// claim stale on arrival lets every relay pass the staleness test on every row.
	t.Run("RefusesAClaimThatIsStaleOnArrival", func(t *testing.T) {
		relay := NewRelay(&fakeStore{}, Recorder(), pry.Recorder(), WithStaleAfter(0))

		if relay.staleAfter < time.Second {
			t.Errorf("staleAfter = %v", relay.staleAfter)
		}
	})

	t.Run("NothingToDo", func(t *testing.T) {
		store := &fakeStore{}
		relay := NewRelay(store, Recorder(), pry.Recorder())

		if relay.pass(context.Background()) {
			t.Fatal("an empty store reported work")
		}
	})

	// The event goes out under the trace it was enqueued with, so the consumer is a
	// child of the request that caused it rather than of this relay pass.
	t.Run("RestoresTheTrace", func(t *testing.T) {
		store := &fakeStore{claim: &Entry{
			Id:            "e1",
			Subject:       "k",
			Payload:       []byte("{}"),
			CorrelationId: "corr-3",
			Traceparent:   parentTrace,
			Tracestate:    "vendor=1",
		}}
		sender := Recorder()
		relay := NewRelay(store, sender, pry.Recorder())

		relay.pass(context.Background())
		sent := sender.Sent()
		if len(sent) != 1 {
			t.Fatalf("sent %d messages, want 1", len(sent))
		}
		if sent[0].CorrelationId != "corr-3" {
			t.Errorf("correlation id: got %q", sent[0].CorrelationId)
		}
		if sent[0].Traceparent != parentTrace {
			t.Errorf("traceparent: got %q, want %q", sent[0].Traceparent, parentTrace)
		}
		if sent[0].Tracestate != "vendor=1" {
			t.Errorf("tracestate: got %q", sent[0].Tracestate)
		}
	})

	// The session value alone is not enough — see restore().
	t.Run("RestoresTheRemoteParent", func(t *testing.T) {
		relay := NewRelay(&fakeStore{}, Recorder(), pry.Recorder())
		ctx := relay.restore(context.Background(), &Entry{Traceparent: parentTrace})

		parent, ok := hilton.RemoteSpanContextFromContext(ctx)
		if !ok {
			t.Fatal("the stored traceparent did not become a remote parent")
		}
		if got := hilton.FormatTraceparent(parent); got != parentTrace {
			t.Errorf("remote parent: got %q, want %q", got, parentTrace)
		}
	})

	t.Run("RecordsAFailedSend", func(t *testing.T) {
		store := &fakeStore{claim: &Entry{Id: "e1", Subject: "k", Payload: []byte("{}")}}
		sender := Recorder()
		sender.FailWith(errRefused)
		relay := NewRelay(store, sender, pry.Recorder())

		if relay.pass(context.Background()) {
			t.Fatal("a refused send reported work done")
		}
		if len(store.failed) != 1 || store.failed[0] != "e1" {
			t.Fatalf("recorded failures: %v", store.failed)
		}
		if len(store.sent) != 0 {
			t.Fatal("a refused entry was marked sent — the outbox would have lost it silently")
		}
		if len(store.causes) != 1 || store.causes[0] != errRefused {
			t.Errorf("recorded cause: %v", store.causes)
		}
	})

	t.Run("SurvivesAPanic", func(t *testing.T) {
		store := &fakeStore{claim: &Entry{Id: "e1", Subject: "k", Payload: []byte("{}")}}
		relay := NewRelay(store, panicSender{}, pry.Recorder())

		if relay.pass(context.Background()) {
			t.Fatal("a pass that panicked reported work done")
		}
	})

	t.Run("StopsOnClose", func(t *testing.T) {
		relay := NewRelay(&fakeStore{}, Recorder(), pry.Recorder(), WithPollInterval(time.Millisecond))

		stopped := make(chan error, 1)
		go func() { stopped <- relay.Run(context.Background()) }()
		if err := relay.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		select {
		case err := <-stopped:
			if err != nil {
				t.Fatalf("run: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the relay did not stop")
		}
		// Close is what the container calls on shutdown, and it may be called twice.
		if err := relay.Close(); err != nil {
			t.Fatalf("second close: %v", err)
		}
	})

	t.Run("StopsOnContext", func(t *testing.T) {
		relay := NewRelay(&fakeStore{}, Recorder(), pry.Recorder(), WithPollInterval(time.Millisecond))

		ctx, cancel := context.WithCancel(context.Background())
		stopped := make(chan error, 1)
		go func() { stopped <- relay.Run(ctx) }()
		cancel()
		select {
		case err := <-stopped:
			if err != nil {
				t.Fatalf("run: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the relay did not stop")
		}
	})
}

type slowSender struct{ take time.Duration }

func (slowSender) Encode(any) ([]byte, error) { return nil, nil }

func (s slowSender) Send(context.Context, string, []byte) error {
	time.Sleep(s.take)
	return nil
}

type panicSender struct{}

func (panicSender) Encode(any) ([]byte, error) { return nil, nil }

func (panicSender) Send(context.Context, string, []byte) error { panic("sender exploded") }
