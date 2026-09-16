package di

import (
	scribus{{ title .Domain }} "{{ .Project }}/domain/{{ .Domain }}/cmd/scribus"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/tools/scribus"
)

func Init{{ title .Domain }}{{ title .Event }}Scribus() {
	di.Wire[scribus.BatchHandler](scribus{{ title .Domain }}.New{{ title .Event }}Command)
	di.Wire[scribus{{ title .Domain }}.{{ title .Domain }}Management]({{ camel .Domain }}ManagementInstance)
}
