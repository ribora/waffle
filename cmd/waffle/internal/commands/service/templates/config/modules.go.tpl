// Composition of waffle modules used by this service.
// Each blank import contributes its di.* registrations to the generated
// container via diogen. Add or remove a line to change the set of modules.
// This file must contain only blank imports — no other code, no other imports.
package di

import (
	_ "gitlab.com/gorib/waffle/config/app"

	// logger
	_ "gitlab.com/gorib/waffle/config/service/pry"

	// observability
	_ "gitlab.com/gorib/waffle/config/service/hilton/otel"

	// uncomment if your service uses redis
	// _ "gitlab.com/gorib/waffle/config/service/redis"

	// uncomment if your service uses elasticsearch
	// _ "gitlab.com/gorib/waffle/config/service/elastic"

	// uncomment if your service uses sql (tracing/metrics are wired automatically)
	// _ "gitlab.com/gorib/waffle/config/service/sql"

	// uncomment if your service uses migrations
	// _ "gitlab.com/gorib/waffle/config/tools/migrate"

	// uncomment if your service uses http server
	// _ "gitlab.com/gorib/waffle/config/tools/http"

	// uncomment if your service uses grpc server
	// _ "gitlab.com/gorib/waffle/config/tools/grpc"

	// uncomment if your service runs scheduled commands in a single long-lived pod
	// (in-process cron). For k8s-native per-task CronJob pods, just register
	// the commands as usual and let k8s schedule them — no import needed.
	// _ "gitlab.com/gorib/waffle/config/tools/cron"

	// uncomment if your service uses scribus — one driver and one mode
	// _ "gitlab.com/gorib/waffle/config/tools/scribus/amqp"
	// CHOOSE ONLY ONE VARIANT either batch processing or single processing
	// _ "gitlab.com/gorib/waffle/config/tools/scribus/batch"
	// _ "gitlab.com/gorib/waffle/config/tools/scribus/single"

	// uncomment if your service publishes to rabbitmq
	// _ "gitlab.com/gorib/waffle/config/service/amqp"

	// uncomment if your published events must survive a crash after the commit —
	// the driver pulls the module in (waffle make:outbox does this and writes the table)
	// _ "gitlab.com/gorib/waffle/config/tools/outbox/amqp"
)
