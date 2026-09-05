package scribus

import (
	"errors"
	"strings"
	"testing"

	"gitlab.com/gorib/amqp"
	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/waffle/app"
)

var _ Consumer = Recorder()

type capturing struct {
	key  string
	seen []string
	fail error
}

func (h *capturing) Keys() []string { return []string{h.key} }

func (h *capturing) Run(src amqp.Source) error {
	h.seen = append(h.seen, string(src.Body()))
	return h.fail
}

type tagging struct {
	order int
	log   *[]string
}

func (m tagging) Name() string { return "tagging" }
func (m tagging) Order() int   { return m.order }

func (m tagging) Wrap(next func(msgs []amqp.Source) error) func(msgs []amqp.Source) error {
	return func(msgs []amqp.Source) error {
		*m.log = append(*m.log, m.Name())
		return next(msgs)
	}
}

func command(t *testing.T, consumer Consumer, handler Handler, middlewares ...Middleware) *scribusCommand {
	t.Helper()
	cmd, err := NewScribusCommand(
		[]Handler{handler}, middlewares,
		app.NewCommand("scribus", "test"), consumer, logger(t),
	)
	if err != nil {
		t.Fatalf("NewScribusCommand: %v", err)
	}
	return cmd
}

func TestBindingsHoldTheHandlerKeys(t *testing.T) {
	consumer := Recorder()
	command(t, consumer, &capturing{key: "contracts.updated"})

	if got := strings.Join(consumer.Bindings(), " "); got != "contracts.updated" {
		t.Fatalf("bindings: %q", got)
	}
}

func TestDeliverReachesTheHandler(t *testing.T) {
	consumer := Recorder()
	handler := &capturing{key: "contracts.updated"}
	command(t, consumer, handler)

	err := consumer.Deliver(t.Context(), amqp.Delivery{
		RoutingKey: "contracts.updated",
		Body:       []byte(`{"id":1}`),
	})

	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if len(handler.seen) != 1 || handler.seen[0] != `{"id":1}` {
		t.Fatalf("seen: %v", handler.seen)
	}
}

func TestDeliverGoesThroughTheMiddlewares(t *testing.T) {
	var passed []string
	consumer := Recorder()
	handler := &capturing{key: "contracts.updated"}
	command(t, consumer, handler, tagging{order: 1, log: &passed})

	if err := consumer.Deliver(t.Context(), amqp.Delivery{RoutingKey: "contracts.updated"}); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	if len(passed) != 1 || passed[0] != "tagging" {
		t.Fatalf("middlewares passed: %v", passed)
	}
	if len(handler.seen) != 1 {
		t.Fatalf("the handler was not reached: %v", handler.seen)
	}
}

func TestDeliverReturnsWhatTheHandlerReturned(t *testing.T) {
	broken := errors.New("database is down")
	consumer := Recorder()
	command(t, consumer, &capturing{key: "contracts.updated", fail: broken})

	err := consumer.Deliver(t.Context(), amqp.Delivery{RoutingKey: "contracts.updated"})

	if !errors.Is(err, broken) {
		t.Fatalf("deliver: %v", err)
	}
}

func TestDeliverSkipsAnUnboundRoutingKey(t *testing.T) {
	consumer := Recorder()
	handler := &capturing{key: "contracts.updated"}
	command(t, consumer, handler)

	if err := consumer.Deliver(t.Context(), amqp.Delivery{RoutingKey: "orders.created"}); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	if len(handler.seen) != 0 {
		t.Fatalf("a message for another key reached the handler: %v", handler.seen)
	}
}

func TestDeliverBeforeBootSaysSo(t *testing.T) {
	err := Recorder().Deliver(t.Context(), amqp.Delivery{RoutingKey: "contracts.updated"})

	if err == nil || !strings.Contains(err.Error(), "no handler installed") {
		t.Fatalf("deliver: %v", err)
	}
}

func TestDeliverPassesEveryMessageAsOnePage(t *testing.T) {
	consumer := Recorder()
	handler := &capturing{key: "contracts.updated"}
	command(t, consumer, handler)

	err := consumer.Deliver(t.Context(),
		amqp.Delivery{RoutingKey: "contracts.updated", Body: []byte("first")},
		amqp.Delivery{RoutingKey: "contracts.updated", Body: []byte("second")},
	)

	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if got := strings.Join(handler.seen, " "); got != "first second" {
		t.Fatalf("seen: %q", got)
	}
}

func TestRunRecordsTheConfigurationAndTheStart(t *testing.T) {
	consumer := Recorder()
	cmd := command(t, consumer, &capturing{key: "contracts.updated"})
	ctx := app.NewContextWithCommand("scribus",
		[]string{"scribus", "amqp://broker", "sherpa", "bastion.guards"}, hilton.Noop(), logger(t))

	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !consumer.DsnSet() {
		t.Error("the dsn did not reach the consumer")
	}
	if consumer.Exchange() != "sherpa" {
		t.Errorf("exchange: %q", consumer.Exchange())
	}
	if consumer.Queue() != "bastion.guards" {
		t.Errorf("queue: %q", consumer.Queue())
	}
	if !consumer.Consuming() {
		t.Error("the command did not get as far as consuming")
	}
}

func TestAPartialConnectionIsRefused(t *testing.T) {
	consumer := Recorder()
	cmd := command(t, consumer, &capturing{key: "contracts.updated"})

	err := usage(t, cmd, "amqp://broker", "sherpa")

	if err == nil || !strings.Contains(err.Error(), "got 2") {
		t.Fatalf("usage: %v", err)
	}
	if consumer.DsnSet() {
		t.Error("the dsn was applied anyway")
	}
}

func TestFlagsReachTheConsumer(t *testing.T) {
	consumer := Recorder()
	cmd := command(t, consumer, &capturing{key: "contracts.updated"})

	if err := usage(t, cmd, "--flush-timeout", "7s", "--reconnect-timeout", "9s"); err != nil {
		t.Fatalf("usage: %v", err)
	}

	if consumer.FlushInterval().String() != "7s" {
		t.Errorf("flush: %s", consumer.FlushInterval())
	}
	if consumer.ReconnectInterval().String() != "9s" {
		t.Errorf("reconnect: %s", consumer.ReconnectInterval())
	}
}
