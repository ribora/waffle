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
	_ "gitlab.com/gorib/waffle/config/service/hilton"

	// uncomment if your service uses sql (tracing/metrics are wired automatically)
	// _ "gitlab.com/gorib/waffle/config/service/sql"

	// uncomment if your service uses migrations
	// _ "gitlab.com/gorib/waffle/config/tools/migrate"

	// uncomment if your service uses http server
	// _ "gitlab.com/gorib/waffle/config/tools/http"

	// uncomment if your service uses grpc server
	// _ "gitlab.com/gorib/waffle/config/tools/grpc"

	// uncomment if your service uses scribus
	// CHOOSE ONLY ONE VARIANT either batch processing or single processing
	// _ "gitlab.com/gorib/waffle/config/tools/scribus/batch"
	// _ "gitlab.com/gorib/waffle/config/tools/scribus/single"
)
