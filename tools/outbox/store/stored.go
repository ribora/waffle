//go:generate go tool store $GOFILE

package store

import (
	"time"

	"gitlab.com/gorib/waffle/tools/outbox"
)

// Id is not readonly: it is a UUIDv7 generated before the insert and goes in with
// the row, which is what lets Add work where a write gives nothing back.
//
// id and claim_token are text rather than a native uuid type, so the table ports to
// a database that has none.
type storedEntry struct {
	_ *outbox.Entry

	Id            string     `db:"id"`
	Subject       string     `db:"subject"`
	Payload       []byte     `db:"payload"`
	CorrelationId string     `db:"correlation_id"`
	Traceparent   string     `db:"traceparent"`
	Tracestate    string     `db:"tracestate"`
	CreatedAt     time.Time  `db:"created_at" store:",readonly"`
	ClaimedAt     *time.Time `db:"claimed_at"`
	ClaimedBy     string     `db:"claimed_by"`
	ClaimToken    *string    `db:"claim_token"`
	SentAt        *time.Time `db:"sent_at"`
	Attempts      int        `db:"attempts"`
	LastError     string     `db:"last_error"`
}
