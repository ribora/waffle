package elastic

import (
	"github.com/ribora/diogen/di"
	"github.com/ribora/elastic"
	"github.com/ribora/env"
	"github.com/ribora/pry"
)

func InitElastic() {
	di.Wire[elastic.Client](func(logger pry.Logger) (elastic.Client, error) {
		host := env.NeedValue[string]("ES_HOST")
		index := env.Value("ES_INDEX", "")
		return elastic.New(host, index, elastic.WithDebugLogger(logger))
	}, di.Fallback())
}
