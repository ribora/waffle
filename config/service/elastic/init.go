package elastic

import (
	"gitlab.com/gorib/di"
	"gitlab.com/gorib/elastic"
	"gitlab.com/gorib/env"
	"gitlab.com/gorib/pry"
)

func InitElastic() {
	di.Wire[elastic.Client](func(logger pry.Logger) (elastic.Client, error) {
		host := env.NeedValue[string]("ES_HOST")
		index := env.Value("ES_INDEX", "")
		return elastic.New(host, index, elastic.WithDebugLogger(logger))
	}, di.Fallback())
}
