package di

import (
	scribus{{ title .Domain }} "{{ .Project }}/domain/{{ .Domain }}/cmd/scribus"

	"gitlab.com/gorib/di"
)

func Init{{ title .Domain }}Scribus() {
	di.Wire[scribus{{ title .Domain }}.{{ title .Domain }}Management]({{ camel .Domain }}ManagementInstance)
}
