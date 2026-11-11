package scribus

import (
	"context"
	"strings"
	"testing"
	"time"

	"gitlab.com/gorib/hilton"
	"gitlab.com/gorib/pry"
	"gitlab.com/gorib/waffle/app"
	"gitlab.com/gorib/whisk/scan"
)

type message struct {
	key  string
	body []byte
}

func (m message) Body() []byte             { return m.body }
func (m message) Context() context.Context { return context.Background() }

func (m message) Lookup(scope, _ string) string {
	if scope == "key" {
		return m.key
	}
	return ""
}

func (m message) LookupAll(scope, key string) []string {
	if v := m.Lookup(scope, key); v != "" {
		return []string{v}
	}
	return nil
}

// A trailing '*' and nothing else: the real semantics are tested in the driver.
type starSubjects struct{}

func (starSubjects) Of(src scan.Source) string { return src.Lookup("key", "") }

func (starSubjects) Match(pattern, subject string) bool {
	if prefix, found := strings.CutSuffix(pattern, "*"); found {
		return strings.HasPrefix(subject, prefix)
	}
	return pattern == subject
}

func (s starSubjects) Overlap(a, b string) bool { return s.Match(a, b) || s.Match(b, a) }

type fakeConsumer struct {
	pageSize uint16
	bound    []string
	applied  []string
	handler  func(msgs []scan.Source) error
}

func (c *fakeConsumer) SetHandler(h func(msgs []scan.Source) error) error {
	c.handler = h
	return nil
}
func (c *fakeConsumer) Params() []string { return []string{"dsn", "exchange", "queue"} }

func (c *fakeConsumer) Apply(args []string) error {
	c.applied = args
	return nil
}

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
func (h oneKeyHandler) Run(scan.Source) error { return nil }

type oneKeyBatch struct{ key string }

func (h oneKeyBatch) Keys() []string          { return []string{h.key} }
func (h oneKeyBatch) Run([]scan.Source) error { return nil }

func logger(*testing.T) pry.Logger {
	return pry.Noop()
}

func single(t *testing.T, consumer Consumer) *scribusCommand {
	t.Helper()
	cmd, err := NewScribusCommand(
		[]Handler{oneKeyHandler{key: "order.created"}}, nil, starSubjects{},
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
		oneKeyBatch{key: "order.created"}, nil, starSubjects{},
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

// The combination compiles and runs, which is why it has to be refused here.
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

// The sizes that give per-message settlement have to stay usable.
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

// Refused while the container is built, so it must not drift into flag parsing.
func TestOverlappingKeysFailAtConstruction(t *testing.T) {
	_, err := NewScribusCommand(
		[]Handler{oneKeyHandler{key: "order.*"}, oneKeyHandler{key: "order.created"}}, nil, starSubjects{},
		app.NewCommand("scribus", "test"), &fakeConsumer{}, logger(t),
	)
	if err == nil {
		t.Fatal("two handlers claiming the same key were accepted")
	}
	if !strings.Contains(err.Error(), "overlap") {
		t.Errorf("error = %v", err)
	}
}
