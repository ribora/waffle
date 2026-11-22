package di

import (
	http{{ title .Domain }} "{{ .Project }}/domain/{{ .Domain }}/cmd/http"

	"github.com/ribora/diogen/di"
	httpCommand "github.com/ribora/waffle/tools/http"
)

func Init{{ title .Domain }}Http() {
	di.Define(http{{ title .Domain }}.NewRouter, di.Tag(httpCommand.RouterTag))
	di.Wire[http{{ title .Domain }}.{{ title .Domain }}Management]({{ camel .Domain }}ManagementInstance)
}
