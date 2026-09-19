package amqp

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/tools/outbox"
	"gitlab.com/gorib/waffle/tools/outbox/rabbit"

	// The publisher and its RABBIT_* live there, so a service that also publishes
	// directly gets one instance on one connection rather than two.
	_ "gitlab.com/gorib/waffle/config/service/amqp"
	// The module this drives: enabling the driver alone builds a Sender nothing
	// consumes, and diogen only warns about it.
	_ "gitlab.com/gorib/waffle/config/tools/outbox"
)

func InitOutboxAmqp() {
	di.Wire[outbox.Sender](rabbit.New, di.Fallback())
}
