package outboxtest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
	"uuid"

	"gitlab.com/gorib/waffle/tools/outbox"
)

// A stale-after of zero puts the cutoff at this instant, so a claim taken a moment
// ago is already stale. That is how these cases reclaim without sleeping.
const reclaimNow = 0

// Longer than any case here, so a claim taken inside one stays taken.
const held = time.Hour

// The requirement outbox.Store cannot state in its signatures: Add joins the
// transaction the context already carries. AddRolledBack catches an implementation
// that does not; the rest are the contract the relay leans on.
type Suite struct {
	// Store returns a Store over an empty outbox. It is called once per case, and
	// clearing what a previous case left behind is its job.
	Store func(t *testing.T) outbox.Store
	// Begin opens a transaction on the same connection the Store writes through and
	// puts it on the context it returns. Without it the case that matters cannot be
	// written at all.
	Begin func(ctx context.Context) (_ context.Context, commit func() error, rollback func() error, _ error)
}

func (s Suite) Run(t *testing.T) {
	if s.Store == nil || s.Begin == nil {
		t.Fatal("outboxtest: both Store and Begin are required")
	}
	t.Run("AddRolledBack", s.addRolledBack)
	t.Run("AddCommitted", s.addCommitted)
	t.Run("ClaimIsExclusive", s.claimIsExclusive)
	t.Run("StaleClaimIsReclaimed", s.staleClaimIsReclaimed)
	t.Run("SentIsFinal", s.sentIsFinal)
	t.Run("FailedKeepsTheClaimAndCounts", s.failedKeepsTheClaimAndCounts)
	t.Run("PurgeRemovesSentOnly", s.purgeRemovesSentOnly)
	t.Run("BacklogReportsTheOldest", s.backlogReportsTheOldest)
}

// The one that matters. A Store that opens its own connection in Add makes the row
// visible here — an event surviving a transaction that never happened.
func (s Suite) addRolledBack(t *testing.T) {
	store := s.Store(t)
	ctx := context.Background()

	tx, _, rollback, err := s.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := store.Add(tx, entry("rolled.back")); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	claimed, err := store.Claim(ctx, "suite", held)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed != nil {
		t.Fatalf("a rolled back entry was claimed: %s", claimed.Subject)
	}
}

func (s Suite) addCommitted(t *testing.T) {
	store := s.Store(t)
	ctx := context.Background()
	want := entry("committed")
	want.CorrelationId = "corr-1"
	want.Traceparent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

	tx, commit, rollback, err := s.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := store.Add(tx, want); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	_ = rollback()

	claimed, err := store.Claim(ctx, "suite", held)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed == nil {
		t.Fatal("a committed entry was not claimed")
	}
	if claimed.Id != want.Id {
		t.Errorf("id: got %q, want %q", claimed.Id, want.Id)
	}
	if claimed.Subject != want.Subject {
		t.Errorf("subject: got %q, want %q", claimed.Subject, want.Subject)
	}
	if string(claimed.Payload) != string(want.Payload) {
		t.Errorf("payload: got %q, want %q", claimed.Payload, want.Payload)
	}
	if claimed.CorrelationId != want.CorrelationId {
		t.Errorf("correlation id: got %q, want %q", claimed.CorrelationId, want.CorrelationId)
	}
	// Restored into the outgoing message by the relay, so the consumer descends
	// from the request that caused the event.
	if claimed.Traceparent != want.Traceparent {
		t.Errorf("traceparent: got %q, want %q", claimed.Traceparent, want.Traceparent)
	}
	if claimed.ClaimedBy != "suite" {
		t.Errorf("claimed by: got %q, want %q", claimed.ClaimedBy, "suite")
	}
	if claimed.CreatedAt.IsZero() {
		t.Error("created at was not filled in")
	}
}

// Two relays, one entry. Without this the outbox delivers duplicates under any
// deployment with more than one replica.
func (s Suite) claimIsExclusive(t *testing.T) {
	store := s.Store(t)
	ctx := context.Background()
	s.add(t, store, entry("contested"))

	var wg sync.WaitGroup
	claimed := make([]*outbox.Entry, 2)
	failed := make([]error, 2)
	start := make(chan struct{})
	for i := range claimed {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			claimed[i], failed[i] = store.Claim(ctx, "suite", held)
		}()
	}
	close(start)
	wg.Wait()

	won := 0
	for i := range claimed {
		if failed[i] != nil {
			t.Fatalf("claim %d: %v", i, failed[i])
		}
		if claimed[i] != nil {
			won++
		}
	}
	if won != 1 {
		t.Fatalf("%d of 2 concurrent claims took the same entry, want exactly 1", won)
	}
}

// A relay that died holding a claim must not take the entry with it.
func (s Suite) staleClaimIsReclaimed(t *testing.T) {
	store := s.Store(t)
	ctx := context.Background()
	s.add(t, store, entry("abandoned"))

	first, err := store.Claim(ctx, "gone", held)
	if err != nil || first == nil {
		t.Fatalf("first claim: %v, %v", first, err)
	}
	again, err := store.Claim(ctx, "waiting", held)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if again != nil {
		t.Fatal("a held claim was taken from under its owner")
	}

	reclaimed, err := store.Claim(ctx, "waiting", reclaimNow)
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if reclaimed == nil {
		t.Fatal("a stale claim was not reclaimed")
	}
	if reclaimed.Id != first.Id {
		t.Errorf("reclaimed a different entry: got %q, want %q", reclaimed.Id, first.Id)
	}
	if reclaimed.ClaimedBy != "waiting" {
		t.Errorf("claimed by: got %q, want %q", reclaimed.ClaimedBy, "waiting")
	}
}

func (s Suite) sentIsFinal(t *testing.T) {
	store := s.Store(t)
	ctx := context.Background()
	s.add(t, store, entry("delivered"))

	claimed, err := store.Claim(ctx, "suite", held)
	if err != nil || claimed == nil {
		t.Fatalf("claim: %v, %v", claimed, err)
	}
	if err := store.Sent(ctx, claimed.Id); err != nil {
		t.Fatalf("sent: %v", err)
	}
	// Even with every claim stale: sent is not a claim state, it is the end.
	again, err := store.Claim(ctx, "suite", reclaimNow)
	if err != nil {
		t.Fatalf("claim after sent: %v", err)
	}
	if again != nil {
		t.Fatalf("a sent entry was claimed again: %s", again.Id)
	}
}

// Failed leaves the claim in place, so a poison entry comes back only once the
// claim goes stale. A Store that released it here would spin the relay.
func (s Suite) failedKeepsTheClaimAndCounts(t *testing.T) {
	store := s.Store(t)
	ctx := context.Background()
	s.add(t, store, entry("refused"))

	claimed, err := store.Claim(ctx, "suite", held)
	if err != nil || claimed == nil {
		t.Fatalf("claim: %v, %v", claimed, err)
	}
	if err := store.Failed(ctx, claimed.Id, errors.New("broker said no")); err != nil {
		t.Fatalf("failed: %v", err)
	}

	immediately, err := store.Claim(ctx, "suite", held)
	if err != nil {
		t.Fatalf("claim after failure: %v", err)
	}
	if immediately != nil {
		t.Fatal("a failed entry was claimable before its claim went stale")
	}

	retried, err := store.Claim(ctx, "suite", reclaimNow)
	if err != nil || retried == nil {
		t.Fatalf("retry claim: %v, %v", retried, err)
	}
	if retried.Attempts != 1 {
		t.Errorf("attempts: got %d, want 1", retried.Attempts)
	}
	if retried.LastError == "" {
		t.Error("last error was not recorded")
	}
}

func (s Suite) purgeRemovesSentOnly(t *testing.T) {
	store := s.Store(t)
	ctx := context.Background()
	s.add(t, store, entry("goes"))

	claimed, err := store.Claim(ctx, "suite", held)
	if err != nil || claimed == nil {
		t.Fatalf("claim: %v, %v", claimed, err)
	}
	if err := store.Sent(ctx, claimed.Id); err != nil {
		t.Fatalf("sent: %v", err)
	}
	s.add(t, store, entry("stays"))

	removed, err := store.Purge(ctx, 0)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if removed != 1 {
		t.Errorf("purged %d entries, want 1", removed)
	}

	left, err := store.Claim(ctx, "suite", reclaimNow)
	if err != nil || left == nil {
		t.Fatalf("claim after purge: %v, %v", left, err)
	}
	if left.Subject != "stays" {
		t.Errorf("purge took the unsent entry: %q is left", left.Subject)
	}
}

// The age of the oldest unsent entry is the one number `outbox status` reports.
func (s Suite) backlogReportsTheOldest(t *testing.T) {
	store := s.Store(t)
	ctx := context.Background()

	empty, err := store.Backlog(ctx)
	if err != nil {
		t.Fatalf("backlog: %v", err)
	}
	if empty.Unsent != 0 || !empty.OldestAt.IsZero() {
		t.Fatalf("an empty outbox reported %d unsent, oldest %v", empty.Unsent, empty.OldestAt)
	}

	first := entry("first")
	s.add(t, store, first)
	s.add(t, store, entry("second"))

	backlog, err := store.Backlog(ctx)
	if err != nil {
		t.Fatalf("backlog: %v", err)
	}
	if backlog.Unsent != 2 {
		t.Errorf("unsent: got %d, want 2", backlog.Unsent)
	}
	if backlog.Failed != 0 {
		t.Errorf("failed: got %d, want 0", backlog.Failed)
	}
	if backlog.OldestAt.IsZero() {
		t.Fatal("oldest was not reported")
	}

	claimed, err := store.Claim(ctx, "suite", held)
	if err != nil || claimed == nil {
		t.Fatalf("claim: %v, %v", claimed, err)
	}
	if claimed.Id != first.Id {
		t.Errorf("claim did not take the oldest entry first: got %q", claimed.Subject)
	}
	if err := store.Sent(ctx, claimed.Id); err != nil {
		t.Fatalf("sent: %v", err)
	}

	after, err := store.Backlog(ctx)
	if err != nil {
		t.Fatalf("backlog after send: %v", err)
	}
	if after.Unsent != 1 {
		t.Errorf("unsent after send: got %d, want 1", after.Unsent)
	}
	if after.OldestAt.IsZero() {
		t.Error("oldest stopped being reported while an entry was still unsent")
	}
	// Not strictly later: how much later depends on the resolution of the column,
	// and a schema whose timestamps tick once a second is still a correct one. The
	// ordering that matters was asserted above, on the id.
	if after.OldestAt.Before(backlog.OldestAt) {
		t.Errorf("oldest went backwards after the first entry was sent: %v then %v", backlog.OldestAt, after.OldestAt)
	}
}

func (s Suite) add(t *testing.T, store outbox.Store, e outbox.Entry) {
	t.Helper()
	if err := store.Add(context.Background(), e); err != nil {
		t.Fatalf("add %s: %v", e.Subject, err)
	}
}

func entry(subject string) outbox.Entry {
	return outbox.Entry{
		Id:      uuid.NewV7().String(),
		Subject: subject,
		Payload: []byte(`{"subject":"` + subject + `"}`),
	}
}
