package scribus

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/ribora/whisk/scan"

	"github.com/ribora/waffle/tools/tap"
)

// No Reset: clearing the wiring would drop the handler the container installed.
func Recorder(params ...string) *ConsumerRecorder {
	return &ConsumerRecorder{params: params}
}

type ConsumerRecorder struct {
	mu        sync.Mutex
	handler   func(msgs []scan.Source) error
	subjects  Subjects
	params    []string
	applied   []string
	bindings  []string
	pageSize  uint16
	flush     time.Duration
	reconnect time.Duration
	consumed  bool
}

func (r *ConsumerRecorder) SetHandler(handler func(msgs []scan.Source) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handler = handler
	return nil
}

func (r *ConsumerRecorder) SetSubjects(subjects Subjects) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subjects = subjects
	return nil
}

func (r *ConsumerRecorder) Params() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.params)
}

func (r *ConsumerRecorder) Apply(args []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(args) != len(r.params) {
		return fmt.Errorf("scribus: the recorder takes %v, got %d values", r.params, len(args))
	}
	r.applied = slices.Clone(args)
	return nil
}

func (r *ConsumerRecorder) Bind(subject string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings = append(r.bindings, subject)
	return nil
}

func (r *ConsumerRecorder) SetPageSize(pageSize uint16) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pageSize = pageSize
	return nil
}

func (r *ConsumerRecorder) SetFlushInterval(interval time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flush = interval
	return nil
}

func (r *ConsumerRecorder) SetReconnectInterval(interval time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reconnect = interval
	return nil
}

// Where the daemon would block; a test uses Deliver instead.
func (r *ConsumerRecorder) Consume(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.consumed = true
	return nil
}

func (r *ConsumerRecorder) Deliver(msgs ...scan.Source) error {
	r.mu.Lock()
	handler, subjects, bindings := r.handler, r.subjects, slices.Clone(r.bindings)
	r.mu.Unlock()

	if handler == nil {
		return errors.New("scribus: no handler installed, boot the container first")
	}
	if subjects != nil {
		for _, msg := range msgs {
			subject := subjects.Of(msg)
			if !slices.ContainsFunc(bindings, func(binding string) bool { return subjects.Match(binding, subject) }) {
				return fmt.Errorf("scribus: no binding takes %q, the consumer binds %q", subject, bindings)
			}
		}
	}
	for _, msg := range msgs {
		if m, ok := msg.(interface {
			RoutingKey() string
			Body() []byte
		}); ok {
			tap.Message("in", m.RoutingKey(), m.Body())
		}
	}
	return handler(msgs)
}

func (r *ConsumerRecorder) Bindings() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.bindings...)
}

// One at a time, and no slice: a failing assertion prints its argument, and one
// of these operands is a connection string with a password in it.
func (r *ConsumerRecorder) Param(name string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	i := slices.Index(r.params, name)
	if i < 0 || i >= len(r.applied) {
		return ""
	}
	return r.applied[i]
}

func (r *ConsumerRecorder) Applied() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.applied) > 0
}

func (r *ConsumerRecorder) PageSize() uint16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pageSize
}

func (r *ConsumerRecorder) FlushInterval() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.flush
}

func (r *ConsumerRecorder) ReconnectInterval() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reconnect
}

func (r *ConsumerRecorder) Consuming() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.consumed
}
