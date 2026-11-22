package rabbit

import (
	"testing"

	"github.com/ribora/amqp"
	"github.com/ribora/waffle/tools/outbox"
)

// New takes an amqp.Publisher, and outbox.Publisher has a Publish of the same
// shape. If it ever grows a Close it becomes an amqp.Publisher too, and
// New(outboxPublisher) starts compiling: a relay that enqueues its own rows and
// delivers nothing, without a word in any log.
func TestNoLoop(t *testing.T) {
	var p outbox.Publisher = outbox.NewPublisher(nil, nil)
	if _, loops := p.(amqp.Publisher); loops {
		t.Fatal("outbox.Publisher satisfies amqp.Publisher — the AMQP adapter would now accept it")
	}
}
