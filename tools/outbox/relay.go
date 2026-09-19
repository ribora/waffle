package outbox

import (
	"context"
	"os"
	"sync"
	"time"

	"gitlab.com/gorib/failure"
	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/session"
)

// The first pause after an idle pass; it doubles up to the poll interval.
const minPollInterval = 10 * time.Millisecond

const minStaleAfter = time.Second

// Two statements: long enough that a slow one still lands, short enough that a
// database that stopped answering does not hold the relay open forever.
const bookkeepingTimeout = 10 * time.Second

type RelayOption func(*relay)

func WithName(name string) RelayOption {
	return func(r *relay) { r.name = name }
}

func WithPollInterval(interval time.Duration) RelayOption {
	return func(r *relay) { r.pollInterval = interval }
}

func WithStaleAfter(stale time.Duration) RelayOption {
	return func(r *relay) { r.staleAfter = stale }
}

func WithRetention(retention time.Duration) RelayOption {
	return func(r *relay) { r.retention = retention }
}

func WithPurgeInterval(interval time.Duration) RelayOption {
	return func(r *relay) { r.purgeInterval = interval }
}

func NewRelay(store Store, sender Sender, logger pry.Logger, opts ...RelayOption) *relay {
	r := &relay{
		store:         store,
		sender:        sender,
		logger:        logger,
		pollInterval:  5 * time.Second,
		staleAfter:    time.Minute,
		retention:     7 * 24 * time.Hour,
		purgeInterval: time.Hour,
		closer:        make(chan struct{}),
	}
	for _, opt := range opts {
		opt(r)
	}
	if r.name == "" {
		r.name, _ = os.Hostname()
	}
	if r.name == "" {
		r.name = "relay"
	}
	// A claim that is stale the moment it is taken is no claim: every relay would
	// pass the staleness test on every row, and two would deliver the same entry.
	// The suite reclaims with zero on purpose, but it calls Store.Claim directly.
	if r.staleAfter < minStaleAfter {
		r.staleAfter = minStaleAfter
	}
	return r
}

type relay struct {
	store  Store
	sender Sender
	logger pry.Logger

	name          string
	pollInterval  time.Duration
	staleAfter    time.Duration
	retention     time.Duration
	purgeInterval time.Duration

	purgeAt time.Time
	closer  chan struct{}
	stopper sync.Once
}

func (r *relay) Run(ctx context.Context) error {
	r.logger.Info("outbox relay started",
		pry.Field("relay", r.name),
		pry.Field("poll", r.pollInterval.String()),
		pry.Field("stale", r.staleAfter.String()),
	)
	defer r.logger.Info("outbox relay stopped", pry.Field("relay", r.name))

	wait := minPollInterval
	for {
		if r.stopping(ctx) {
			return nil
		}
		r.purgeIfDue(ctx)
		if r.pass(ctx) {
			wait = minPollInterval
			continue
		}
		if wait *= 2; wait > r.pollInterval {
			wait = r.pollInterval
		}
		select {
		case <-ctx.Done():
			return nil
		case <-r.closer:
			return nil
		case <-time.After(wait):
		}
	}
}

func (r *relay) Close() error {
	r.stopper.Do(func() { close(r.closer) })
	return nil
}

func (r *relay) stopping(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	case <-r.closer:
		return true
	default:
		return false
	}
}

// A panic on one entry must not take the relay down: the rest of the backlog is
// still deliverable.
func (r *relay) pass(ctx context.Context) (worked bool) {
	if err := failure.Catch(func() error {
		worked = r.deliver(ctx)
		return nil
	}); err != nil {
		r.logger.Error(err, pry.Ctx(ctx), pry.Field("relay", r.name))
		return false
	}
	return worked
}

func (r *relay) deliver(ctx context.Context) bool {
	entry, err := r.store.Claim(ctx, r.name, r.staleAfter)
	if err != nil {
		r.logger.Error("outbox: cannot claim an entry", pry.Err(err), pry.Ctx(ctx))
		return false
	}
	if entry == nil {
		return false
	}

	send := r.restore(ctx, entry)
	sendErr := r.sender.Send(send, entry.Subject, entry.Payload)

	// The bookkeeping does not take the caller's cancellation: an entry the broker
	// has already accepted has to be marked whatever happened to ctx, or the next
	// pass sends it a second time. Its own deadline, because dropping the caller's
	// leaves none — gorib/sql sets no timeout on a query.
	book, cancel := context.WithTimeout(context.WithoutCancel(ctx), bookkeepingTimeout)
	defer cancel()

	if sendErr != nil {
		r.logger.Error("outbox: cannot send an entry", pry.Err(sendErr), pry.Ctx(send), pry.Field("entry", entry.Id))
		// Failed keeps the claim, so this row is retried only after staleAfter — a
		// message the broker keeps refusing cannot spin this loop.
		if err := r.store.Failed(book, entry.Id, sendErr); err != nil {
			r.logger.Error("outbox: cannot record a failed entry", pry.Err(err), pry.Ctx(ctx), pry.Field("entry", entry.Id))
		}
		return false
	}
	if err := r.store.Sent(book, entry.Id); err != nil {
		// Delivered but still claimed, so it goes out again after staleAfter.
		r.logger.Error("outbox: cannot mark an entry sent", pry.Err(err), pry.Ctx(ctx), pry.Field("entry", entry.Id))
	}
	return true
}

func (r *relay) restore(ctx context.Context, entry *Entry) context.Context {
	ctx = session.NewContextWithSession(ctx, session.Session{
		CorrelationId: entry.CorrelationId,
		Method:        "OUTBOX",
		Uri:           entry.Subject,
	})
	if entry.Traceparent == "" && entry.Tracestate == "" {
		return ctx
	}
	ctx = session.WithTrace(ctx, entry.Traceparent, entry.Tracestate)
	// Not redundant with the line above: the sender starts its own span, which
	// without a remote parent is a root, and the stored trace is lost.
	if sc, ok := hilton.ParseTraceparent(entry.Traceparent); ok {
		ctx = hilton.ContextWithRemoteSpanContext(ctx, sc)
	}
	return ctx
}

func (r *relay) purgeIfDue(ctx context.Context) {
	if r.retention <= 0 || time.Now().Before(r.purgeAt) {
		return
	}
	r.purgeAt = time.Now().Add(r.purgeInterval)
	removed, err := r.store.Purge(ctx, r.retention)
	if err != nil {
		r.logger.Error("outbox: cannot purge sent entries", pry.Err(err), pry.Ctx(ctx))
		return
	}
	if removed > 0 {
		r.logger.Info("outbox: purged sent entries", pry.Field("removed", removed))
	}
}
