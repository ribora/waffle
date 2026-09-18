package di

import (
	http{{ title .Domain }} "{{ .Project }}/domain/{{ .Domain }}/cmd/http"

	"gitlab.com/gorib/di"
	httpCommand "gitlab.com/gorib/waffle/tools/http"
)

func Init{{ title .Domain }}Http() {
	di.Define(http{{ title .Domain }}.NewRouter, di.Tag(httpCommand.RouterTag))
	di.Wire[http{{ title .Domain }}.{{ title .Domain }}Management]({{ camel .Domain }}ManagementInstance)
}
