package scribus

import (
	"context"
	"strings"
	"testing"
	"time"

	"gitlab.com/gorib/amqp"
	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/waffle/app"
)

type fakeConsumer struct {
	pageSize uint16
	bound    []string
	handler  func(msgs []amqp.Source) error
}

func (c *fakeConsumer) SetHandler(h func(msgs []amqp.Source) error) error {
	c.handler = h
	return nil
}
func (c *fakeConsumer) SetDsn(string) error      { return nil }
func (c *fakeConsumer) SetExchange(string) error { return nil }
func (c *fakeConsumer) SetQueue(string) error    { return nil }
func (c *fakeConsumer) Bind(key string) error {
	c.bound = append(c.bound, key)
	return nil
}
func (c *fakeConsumer) SetPageSize(n uint16) error {
	c.pageSize = n
	return nil
}
func (c *fakeConsumer) SetFlushInterval(time.Duration) error     { return nil }
func (c *fakeConsumer) SetReconnectInterval(time.Duration) error { return nil }
func (c *fakeConsumer) Consume(context.Context) error            { return nil }

type oneKeyHandler struct{ key string }

func (h oneKeyHandler) Keys() []string        { return []string{h.key} }
func (h oneKeyHandler) Run(amqp.Source) error { return nil }

type oneKeyBatch struct{ key string }

func (h oneKeyBatch) Keys() []string          { return []string{h.key} }
func (h oneKeyBatch) Run([]amqp.Source) error { return nil }

func logger(t *testing.T) pry.Logger {
	t.Helper()
	l, err := pry.New("warn")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func single(t *testing.T, consumer Consumer) *scribusCommand {
	t.Helper()
	cmd, err := NewScribusCommand(
		[]Handler{oneKeyHandler{key: "order.created"}}, nil,
		app.NewCommand("scribus", "test"), consumer, logger(t),
	)
	if err != nil {
		t.Fatalf("NewScribusCommand: %v", err)
	}
	return cmd
}

func batch(t *testing.T, consumer Consumer) *scribusCommand {
	t.Helper()
	cmd, err := NewBatchScribusCommand(
		oneKeyBatch{key: "order.created"}, nil,
		app.NewCommand("scribus", "test"), consumer, logger(t),
	)
	if err != nil {
		t.Fatalf("NewBatchScribusCommand: %v", err)
	}
	return cmd
}

// The command name leads, as it does when app.Run hands the rest to getopt.
func usage(t *testing.T, cmd *scribusCommand, args ...string) error {
	t.Helper()
	ctx := app.NewContextWithCommand("scribus", append([]string{"scribus"}, args...), hilton.Noop(), logger(t))
	return cmd.Usage(ctx)
}

// A page settles as one unit, and only a BatchHandler is written for that. The
// combination compiles and runs, which is why it has to be refused here.
func TestPageAboveOneNeedsABatchHandler(t *testing.T) {
	consumer := &fakeConsumer{}
	err := usage(t, single(t, consumer), "--page", "50")
	if err == nil {
		t.Fatal("single mode accepted a page size that settles messages in groups")
	}
	for _, want := range []string{"--page 50", "BatchHandler"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if consumer.pageSize != 0 {
		t.Errorf("the page size was applied anyway: %d", consumer.pageSize)
	}
}

func TestPageAboveOneIsFineInBatchMode(t *testing.T) {
	consumer := &fakeConsumer{}
	if err := usage(t, batch(t, consumer), "--page", "50"); err != nil {
		t.Fatalf("batch mode refused its own flag: %v", err)
	}
	if consumer.pageSize != 50 {
		t.Errorf("page size = %d, want 50", consumer.pageSize)
	}
}

// Per-message settlement is the whole point of single mode, so the sizes that
// give it have to stay usable.
func TestSingleModeAcceptsPerMessagePages(t *testing.T) {
	for _, args := range [][]string{{}, {"--page", "1"}} {
		consumer := &fakeConsumer{}
		if err := usage(t, single(t, consumer), args...); err != nil {
			t.Errorf("single mode refused %v: %v", args, err)
		}
	}
}

// An absent flag leaves the size to the container, so Usage must not touch it.
func TestAbsentPageLeavesTheConfiguredSize(t *testing.T) {
	consumer := &fakeConsumer{}
	if err := usage(t, batch(t, consumer)); err != nil {
		t.Fatalf("usage: %v", err)
	}
	if consumer.pageSize != 0 {
		t.Errorf("SetPageSize was called with %d", consumer.pageSize)
	}
}

// Overlapping keys are refused while the container is built — before any
// connection — so this failure must not drift into flag parsing.
func TestOverlappingKeysFailAtConstruction(t *testing.T) {
	_, err := NewScribusCommand(
		[]Handler{oneKeyHandler{key: "order.*"}, oneKeyHandler{key: "order.created"}}, nil,
		app.NewCommand("scribus", "test"), &fakeConsumer{}, logger(t),
	)
	if err == nil {
		t.Fatal("two handlers claiming the same key were accepted")
	}
	if !strings.Contains(err.Error(), "overlap") {
		t.Errorf("error = %v", err)
	}
}
