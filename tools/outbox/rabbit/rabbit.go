package rabbit

import (
	"context"
	"encoding/json"

	"gitlab.com/gorib/amqp"
)

// Nothing to add around the publisher: it already carries session and trace out
// of ctx, and its Publish returns only once the broker has confirmed — which is
// the one thing Sender asks.
func New(publisher amqp.Publisher) *sender {
	return &sender{publisher: publisher}
}

type sender struct {
	publisher amqp.Publisher
}

func (s *sender) Encode(document any) ([]byte, error) {
	return json.Marshal(document)
}

// EncodedMessage, so the payload is not marshalled a second time: it was encoded
// once already, at Enqueue.
func (s *sender) Send(ctx context.Context, subject string, payload []byte) error {
	return s.publisher.Publish(ctx, subject, amqp.EncodedMessage(payload))
}
