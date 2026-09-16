package di

import (
	http{{ title .Domain }} "{{ .Project }}/domain/{{ .Domain }}/cmd/http"

	"gitlab.com/gorib/di"
	httpServer "gitlab.com/gorib/http/server"
)

func Init{{ title .Domain }}Http() {
	di.Wire[httpServer.Router](http{{ title .Domain }}.NewRouter)
	di.Wire[http{{ title .Domain }}.{{ title .Domain }}Management]({{ camel .Domain }}ManagementInstance)
}
