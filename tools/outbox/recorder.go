package outbox

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/ribora/session"
)

// Replace the Sender and not the Store, or Enqueue stops writing the row the
// scenario is about.
func Recorder() *recorder { return &recorder{} }

type Sent struct {
	Subject       string
	Payload       []byte
	CorrelationId string
	Traceparent   string
	Tracestate    string
}

type recorder struct {
	mu   sync.Mutex
	sent []Sent
	err  error
}

func (r *recorder) FailWith(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

func (r *recorder) Encode(document any) ([]byte, error) {
	return json.Marshal(document)
}

func (r *recorder) Send(ctx context.Context, subject string, payload []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	record := Sent{Subject: subject, Payload: payload}
	if s := session.FromContext(ctx); s != nil {
		record.CorrelationId = s.CorrelationId
	}
	record.Traceparent, record.Tracestate = session.TraceFromContext(ctx)
	r.sent = append(r.sent, record)
	return nil
}

func (r *recorder) Sent() []Sent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Sent(nil), r.sent...)
}

func (r *recorder) WithSubject(subject string) []Sent {
	r.mu.Lock()
	defer r.mu.Unlock()
	var found []Sent
	for _, s := range r.sent {
		if s.Subject == subject {
			found = append(found, s)
		}
	}
	return found
}

func (r *recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent, r.err = nil, nil
}
