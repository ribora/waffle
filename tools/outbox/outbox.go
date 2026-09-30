package outbox

import (
	"context"
	"time"
	"uuid"

	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/session"

	"gitlab.com/gorib/waffle/tools/trace"
)

const instrumentationName = "gitlab.com/gorib/waffle/tools/outbox"

// Send must return an error when the broker has not confirmed delivery: the relay
// marks the row sent on its word.
type Sender interface {
	Encode(document any) ([]byte, error)
	Send(ctx context.Context, subject string, payload []byte) error
}

// Id is a UUIDv7 and the relay claims in id order, so anything else there silently
// reorders delivery.
//
// ClaimedAt and ClaimToken are nobody's business but a Store's, and dropping them
// from here was tried: the store generator refuses a column with no field behind
// it, and both columns have to stay addressable for Claim to query them.
type Entry struct {
	Id            string
	Subject       string
	Payload       []byte
	CorrelationId string
	Traceparent   string
	Tracestate    string
	CreatedAt     time.Time
	ClaimedAt     *time.Time
	ClaimedBy     string
	ClaimToken    *string
	SentAt        *time.Time
	Attempts      int
	LastError     string
}

type Backlog struct {
	Unsent int
	Failed int
	// The store's clock, which `outbox status` subtracts from the pod's. Skew shows
	// up in the age it prints, and enough of it prints a negative one.
	OldestAt time.Time
}

// The one requirement that cannot be written below: Add must join whatever
// transaction the context already carries. Run outboxtest.Suite against it.
type Store interface {
	Add(ctx context.Context, entry Entry) error
	// Claim returns nil, nil when there is nothing to claim.
	Claim(ctx context.Context, by string, staleAfter time.Duration) (*Entry, error)
	Sent(ctx context.Context, id string) error
	Failed(ctx context.Context, id string, cause error) error
	Purge(ctx context.Context, olderThan time.Duration) (int, error)
	Backlog(ctx context.Context) (Backlog, error)
	// Reset clears attempts and last_error. An empty id takes every entry that ran
	// out; an id takes that one whatever its attempts. Neither revives a sent entry.
	Reset(ctx context.Context, id string) (int, error)
}

type Relay interface {
	Run(ctx context.Context) error
	Close() error
}

// The method name is the guarantee: Publish goes now, Enqueue writes a row in the
// caller's transaction.
type Publisher interface {
	Publish(ctx context.Context, subject string, document any) error
	Enqueue(ctx context.Context, subject string, document any) error
}

// No Close: it would make this an amqp.Publisher, which the AMQP adapter accepts —
// a relay writing rows instead of sending them. Pinned by rabbit.TestNoLoop.
func NewPublisher(store Store, sender Sender) *publisher {
	return &publisher{store: store, sender: sender}
}

type publisher struct {
	store  Store
	sender Sender
}

func (p *publisher) Publish(ctx context.Context, subject string, document any) error {
	payload, err := p.sender.Encode(document)
	if err != nil {
		return err
	}
	return p.sender.Send(ctx, subject, payload)
}

func (p *publisher) Enqueue(ctx context.Context, subject string, document any) (err error) {
	provider := hilton.ProviderFromContext(ctx)
	ctx, span := provider.Tracer(instrumentationName).Start(ctx, "outbox enqueue",
		hilton.WithSpanKind(hilton.SpanKindProducer),
		hilton.WithAttributes(
			hilton.Attr("messaging.operation.type", "create"),
			hilton.Attr("messaging.destination.name", subject),
		),
	)
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(hilton.StatusError, err.Error())
		}
	}()
	// This span's traceparent is what gets stored, not the caller's: the relay
	// restores it, so the consumer descends from the request that caused the event.
	if tp := hilton.FormatTraceparent(span.SpanContext()); tp != "" {
		ctx = session.WithTraceparent(ctx, tp)
	}

	payload, err := p.sender.Encode(document)
	if err != nil {
		return err
	}
	entry := Entry{
		Id:      uuid.NewV7().String(),
		Subject: subject,
		Payload: payload,
	}
	if s := session.FromContext(ctx); s != nil {
		entry.CorrelationId = s.CorrelationId
	}
	entry.Traceparent, entry.Tracestate = session.TraceFromContext(ctx)
	if err := p.store.Add(ctx, entry); err != nil {
		return err
	}
	trace.Event("out", subject, payload)
	return nil
}
