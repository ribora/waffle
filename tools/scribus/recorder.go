package scribus

import (
	"context"
	"errors"
	"sync"
	"time"

	"gitlab.com/gorib/amqp"
)

// A silent consumer swallows Bind, so a typo in a routing key stays green while
// production receives nothing. No Reset: clearing the wiring would drop the
// installed handler. Take a new one per scenario.
func Recorder() *ConsumerRecorder { return &ConsumerRecorder{} }

type ConsumerRecorder struct {
	mu        sync.Mutex
	handler   func(msgs []amqp.Source) error
	dsn       string
	exchange  string
	queue     string
	bindings  []string
	pageSize  uint16
	flush     time.Duration
	reconnect time.Duration
	consumed  bool
}

func (r *ConsumerRecorder) SetHandler(handler func(msgs []amqp.Source) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handler = handler
	return nil
}

func (r *ConsumerRecorder) SetDsn(dsn string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dsn = dsn
	return nil
}

func (r *ConsumerRecorder) SetExchange(exchange string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exchange = exchange
	return nil
}

func (r *ConsumerRecorder) SetQueue(queue string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queue = queue
	return nil
}

func (r *ConsumerRecorder) Bind(routingKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings = append(r.bindings, routingKey)
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

// One page, the way the broker would. Returns what the handler returned: nil
// settled, error requeued.
func (r *ConsumerRecorder) Deliver(ctx context.Context, deliveries ...amqp.Delivery) error {
	r.mu.Lock()
	handler := r.handler
	r.mu.Unlock()

	if handler == nil {
		return errors.New("scribus: no handler installed, boot the container first")
	}
	msgs := make([]amqp.Source, 0, len(deliveries))
	for _, delivery := range deliveries {
		msgs = append(msgs, delivery.Source(ctx))
	}
	return handler(msgs)
}

func (r *ConsumerRecorder) Bindings() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.bindings...)
}

func (r *ConsumerRecorder) Exchange() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.exchange
}

func (r *ConsumerRecorder) Queue() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.queue
}

// Whether a DSN reached the consumer, not which one: a broker DSN carries a
// password, and a failing assertion prints its argument into the CI log.
func (r *ConsumerRecorder) DsnSet() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dsn != ""
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
