package scribus

import (
	"errors"
	"strings"
	"testing"

	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/waffle/app"
	"gitlab.com/gorib/whisk/scan"
)

var _ Consumer = Recorder()

type capturing struct {
	key  string
	seen []string
	fail error
}

func (h *capturing) Keys() []string { return []string{h.key} }

func (h *capturing) Run(src scan.Source) error {
	h.seen = append(h.seen, string(src.Body()))
	return h.fail
}

type tagging struct {
	order int
	log   *[]string
}

func (m tagging) Name() string { return "tagging" }
func (m tagging) Order() int   { return m.order }

func (m tagging) Wrap(next func(msgs []scan.Source) error) func(msgs []scan.Source) error {
	return func(msgs []scan.Source) error {
		*m.log = append(*m.log, m.Name())
		return next(msgs)
	}
}

func command(t *testing.T, consumer Consumer, handler Handler, middlewares ...Middleware) *scribusCommand {
	t.Helper()
	cmd, err := NewScribusCommand(
		[]Handler{handler}, middlewares, starSubjects{},
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

	err := consumer.Deliver(message{key: "contracts.updated", body: []byte(`{"id":1}`)})

	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if len(handler.seen) != 1 || handler.seen[0] != `{"id":1}` {
		t.Fatalf("seen: %v", handler.seen)
	}
}

// A subject the handler never spelled out, so only Match can route it.
func TestDeliverAsksTheDriverToMatch(t *testing.T) {
	consumer := Recorder()
	handler := &capturing{key: "orders.*"}
	command(t, consumer, handler)

	if err := consumer.Deliver(message{key: "orders.created", body: []byte("first")}); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	if len(handler.seen) != 1 || handler.seen[0] != "first" {
		t.Fatalf("a message matching the bound pattern did not reach the handler: %v", handler.seen)
	}
}

func TestDeliverGoesThroughTheMiddlewares(t *testing.T) {
	var passed []string
	consumer := Recorder()
	handler := &capturing{key: "contracts.updated"}
	command(t, consumer, handler, tagging{order: 1, log: &passed})

	if err := consumer.Deliver(message{key: "contracts.updated"}); err != nil {
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

	err := consumer.Deliver(message{key: "contracts.updated"})

	if !errors.Is(err, broken) {
		t.Fatalf("deliver: %v", err)
	}
}

func TestDeliverSkipsAnUnboundSubject(t *testing.T) {
	consumer := Recorder()
	handler := &capturing{key: "contracts.updated"}
	command(t, consumer, handler)

	if err := consumer.Deliver(message{key: "orders.created"}); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	if len(handler.seen) != 0 {
		t.Fatalf("a message for another key reached the handler: %v", handler.seen)
	}
}

// Trace is a per-service toggle, not a per-line one, so turning it on to chase a
// dispatch problem must not put every payload in the sink.
func TestTraceNamesTheSubjectAndNotTheBody(t *testing.T) {
	log := pry.Recorder()
	consumer := Recorder()
	if _, err := NewScribusCommand(
		[]Handler{&capturing{key: "contracts.updated"}}, nil, starSubjects{},
		app.NewCommand("scribus", "test"), consumer, log,
	); err != nil {
		t.Fatalf("NewScribusCommand: %v", err)
	}

	if err := consumer.Deliver(message{key: "contracts.updated", body: []byte(`{"iban":"DE89"}`)}); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	traced := log.At(pry.LevelTrace)
	if len(traced) != 1 {
		t.Fatalf("trace records: %v", traced)
	}
	if traced[0].Message != "contracts.updated" {
		t.Errorf("trace message = %q, want the subject alone", traced[0].Message)
	}
	for _, record := range log.Entries() {
		if strings.Contains(record.Message, "iban") {
			t.Errorf("a log record carries the payload: %q", record.Message)
		}
	}
}

func TestDeliverBeforeBootSaysSo(t *testing.T) {
	err := Recorder().Deliver(message{key: "contracts.updated"})

	if err == nil || !strings.Contains(err.Error(), "no handler installed") {
		t.Fatalf("deliver: %v", err)
	}
}

func TestDeliverPassesEveryMessageAsOnePage(t *testing.T) {
	consumer := Recorder()
	handler := &capturing{key: "contracts.updated"}
	command(t, consumer, handler)

	err := consumer.Deliver(
		message{key: "contracts.updated", body: []byte("first")},
		message{key: "contracts.updated", body: []byte("second")},
	)

	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if got := strings.Join(handler.seen, " "); got != "first second" {
		t.Fatalf("seen: %q", got)
	}
}

func TestRunRecordsTheConfigurationAndTheStart(t *testing.T) {
	consumer := Recorder("dsn", "exchange", "queue")
	cmd := command(t, consumer, &capturing{key: "contracts.updated"})
	ctx := app.NewContextWithCommand("scribus",
		[]string{"scribus", "amqp://broker", "sherpa", "bastion.guards"}, hilton.Noop(), logger(t))

	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !consumer.Applied() {
		t.Error("the connection operands did not reach the consumer")
	}
	if consumer.Param("exchange") != "sherpa" {
		t.Errorf("exchange: %q", consumer.Param("exchange"))
	}
	if consumer.Param("queue") != "bastion.guards" {
		t.Errorf("queue: %q", consumer.Param("queue"))
	}
	if !consumer.Consuming() {
		t.Error("the command did not get as far as consuming")
	}
}

func TestAPartialConnectionIsRefused(t *testing.T) {
	consumer := Recorder("dsn", "exchange", "queue")
	cmd := command(t, consumer, &capturing{key: "contracts.updated"})

	err := usage(t, cmd, "amqp://broker", "sherpa")

	if err == nil || !strings.Contains(err.Error(), "got 2") {
		t.Fatalf("usage: %v", err)
	}
	if !strings.Contains(err.Error(), "dsn exchange queue") {
		t.Errorf("the error does not name the operands the driver takes: %v", err)
	}
	if consumer.Applied() {
		t.Error("the operands were applied anyway")
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
