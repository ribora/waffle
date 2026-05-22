//go:generate go tool diogen -output config/diogen/di_gen.go -pkg diogen $GOFILE
package di

import (
	"gitlab.com/gorib/di"
)

func InitDi() {
	di.Include("gitlab.com/gorib/waffle/config/app")

	// logger
	di.Include("gitlab.com/gorib/waffle/config/service/pry")

	// observability
	di.Include("gitlab.com/gorib/waffle/config/service/hilton")

	// select your sql db: CHOOSE ONLY ONE VARIANT
	// di.Include("gitlab.com/gorib/waffle/config/service/sql")
	// di.Include("gitlab.com/gorib/waffle/config/service/sql/tracing")

	// uncomment if your service uses migrations
	// di.Include("gitlab.com/gorib/waffle/config/tools/migrate")

	// uncomment if your service uses http server
	// di.Include("gitlab.com/gorib/waffle/config/tools/http")

	// uncomment if your service uses grpc server
	// di.Include("gitlab.com/gorib/waffle/config/tools/grpc")

	// uncomment if your service uses scribus
	// CHOOSE ONLY ONE VARIANT either batch processing or single processing
	// di.Include("gitlab.com/gorib/waffle/config/tools/scribus/batch")
	// di.Include("gitlab.com/gorib/waffle/config/tools/scribus/single")
}
