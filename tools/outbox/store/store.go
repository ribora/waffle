package store

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/ribora/criteria"
	"github.com/ribora/sql"
	"github.com/ribora/waffle/tools/outbox"
)

// A name, not a package-level sql.Table the way a generated repository holds one:
// the builder writes into the *from it is handed, so a shared one is a data race.
const table = "outbox"

const maxLastError = 1024

type Option func(*store)

func WithMaxAttempts(attempts int) Option {
	return func(s *store) { s.maxAttempts = attempts }
}

func NewStore(db sql.Db, opts ...Option) (*store, error) {
	repo, err := sql.NewRepository[storedEntry](db)
	if err != nil {
		return nil, err
	}
	s := &store{repo: repo, maxAttempts: 10}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

type store struct {
	repo        sql.Repository[storedEntry]
	maxAttempts int
}

// The guarantee is the repository's: it runs on the transaction the context
// carries and opens its own only when there is none.
func (s *store) Add(ctx context.Context, entry outbox.Entry) error {
	fields, values := newStoredEntry(&entry).Inserts()
	_, err := s.repo.Select(ctx, sql.Insert(fields...).Into(sql.Table(table)).Values(values...))
	return err
}

// Two statements instead of one UPDATE ... RETURNING, which MySQL does not have:
// the update stamps a token, and the select finds the row only if this attempt won.
func (s *store) Claim(ctx context.Context, by string, staleAfter time.Duration) (*outbox.Entry, error) {
	now := time.Now()
	token := uuid.NewV7().String()
	unclaimed := criteria.Or(sql.Eq("claimedAt", nil), sql.Lt("claimedAt", now.Add(-staleAfter)))

	oldest := sql.Select("id").From(sql.Table(table)).
		Where(criteria.And(
			sql.Eq("sentAt", nil),
			sql.Lt("attempts", s.maxAttempts),
			unclaimed,
		)).
		Sort("id").Page(0, 1)
	// Wrapped in a derived table: MySQL refuses to read the table an UPDATE targets
	// from a subquery in its own WHERE (error 1093). Postgres does not need it.
	candidate := sql.Select("id").From(sql.Query(oldest, "candidate"))

	// Repeating the conditions here is the whole locking scheme: two relays can pick
	// the same id, the row lock serialises them, and the loser re-evaluates this
	// WHERE against the winner's row and updates nothing.
	claim := sql.Update(map[string]any{
		"claimed_at":  now,
		"claimed_by":  by,
		"claim_token": token,
	}).Table(sql.Table(table)).Where(criteria.And(sql.Eq("id", candidate), sql.Eq("sentAt", nil), unclaimed))
	if _, err := s.repo.Select(ctx, claim); err != nil {
		return nil, err
	}

	claimed, err := s.repo.Get(ctx, sql.Select().From(sql.Table(table)).Where(criteria.And(sql.Eq("claimToken", token))))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return claimed.Unwrap(), nil
}

func (s *store) Sent(ctx context.Context, id string) error {
	_, err := s.repo.Select(ctx, sql.Update(map[string]any{"sent_at": time.Now()}).
		Table(sql.Table(table)).Where(criteria.And(sql.Eq("id", id))))
	return err
}

// claimed_at is left in place on purpose: the entry returns only once the claim
// goes stale, which is what stops a poison entry spinning the relay.
//
// Read then write, because attempts+1 has no expression in the builder. Safe: the
// caller holds the claim.
func (s *store) Failed(ctx context.Context, id string, cause error) error {
	return s.inTransaction(ctx, func(ctx context.Context) error {
		entry, err := s.repo.Get(ctx, sql.Select().From(sql.Table(table)).Where(criteria.And(sql.Eq("id", id))))
		if err != nil {
			return err
		}
		message := ""
		if cause != nil {
			message = truncate(cause.Error(), maxLastError)
		}
		_, err = s.repo.Select(ctx, sql.Update(map[string]any{
			"attempts":   entry.Attempts + 1,
			"last_error": message,
		}).Table(sql.Table(table)).Where(criteria.And(sql.Eq("id", id))))
		return err
	})
}

func (s *store) Purge(ctx context.Context, olderThan time.Duration) (int, error) {
	sent := criteria.And(sql.Ne("sentAt", nil), sql.Lt("sentAt", time.Now().Add(-olderThan)))
	var removed int
	err := s.inTransaction(ctx, func(ctx context.Context) error {
		// Counted first: Repository.Delete gives no rows-affected back.
		found, err := s.repo.Count(ctx, sql.Count().From(sql.Table(table)).Where(sent))
		if err != nil || found == 0 {
			return err
		}
		if err := s.repo.Delete(ctx, sql.Delete().From(sql.Table(table)).Where(sent)); err != nil {
			return err
		}
		removed = found
		return nil
	})
	return removed, err
}

func (s *store) Backlog(ctx context.Context) (outbox.Backlog, error) {
	var backlog outbox.Backlog
	unsent := sql.Eq("sentAt", nil)
	err := s.inTransaction(ctx, func(ctx context.Context) error {
		total, err := s.repo.Count(ctx, sql.Count().From(sql.Table(table)).Where(criteria.And(unsent)))
		if err != nil {
			return err
		}
		exhausted, err := s.repo.Count(ctx, sql.Count().From(sql.Table(table)).
			Where(criteria.And(unsent, sql.Ge("attempts", s.maxAttempts))))
		if err != nil {
			return err
		}
		backlog.Unsent, backlog.Failed = total, exhausted
		if total == 0 {
			return nil
		}
		// min(created_at) has no expression in the builder, and does not need one:
		// the id is a UUIDv7, so the first row by id is the first row by time.
		oldest, err := s.repo.Get(ctx, sql.Select().From(sql.Table(table)).
			Where(criteria.And(unsent)).Sort("id").Page(0, 1))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}
		backlog.OldestAt = oldest.CreatedAt
		return nil
	})
	return backlog, err
}

// Reset also drops the claim, so a fixed entry goes out on the next pass instead
// of waiting for the claim it still holds to go stale.
func (s *store) Reset(ctx context.Context, id string) (int, error) {
	where := criteria.And(sql.Eq("sentAt", nil), sql.Ge("attempts", s.maxAttempts))
	if id != "" {
		// Whatever its attempts, but a sent entry is never resurrected.
		where = criteria.And(sql.Eq("id", id), sql.Eq("sentAt", nil))
	}
	var reset int
	err := s.inTransaction(ctx, func(ctx context.Context) error {
		found, err := s.repo.Count(ctx, sql.Count().From(sql.Table(table)).Where(where))
		if err != nil || found == 0 {
			return err
		}
		if _, err := s.repo.Select(ctx, sql.Update(map[string]any{
			"attempts":    0,
			"last_error":  "",
			"claimed_at":  nil,
			"claimed_by":  "",
			"claim_token": nil,
		}).Table(sql.Table(table)).Where(where)); err != nil {
			return err
		}
		reset = found
		return nil
	})
	return reset, err
}

// The repository's own rule for one statement, applied to a pair: run on the
// caller's transaction, or open and end one here.
func (s *store) inTransaction(ctx context.Context, fn func(context.Context) error) (err error) {
	if _, err := sql.TransactionFromContext(ctx); err == nil {
		return fn(ctx)
	}
	ctx, commit, rollback, err := s.repo.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			err = commit()
		}
		// commit() marks the transaction done only when it succeeds, so this
		// rollback is what closes one whose commit failed.
		if e := rollback(); err == nil {
			err = e
		}
	}()
	return fn(ctx)
}

// Back off to a rune boundary, or last_error is invalid UTF-8 and the whole of
// Failed rolls back — leaving the entry at its old attempts, to be claimed again
// forever. A poison message is exactly the one that produces the long error.
func truncate(message string, limit int) string {
	if len(message) <= limit {
		return message
	}
	for limit > 0 && !utf8.RuneStart(message[limit]) {
		limit--
	}
	return message[:limit]
}
