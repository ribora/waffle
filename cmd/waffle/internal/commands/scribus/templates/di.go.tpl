package di

import (
	scribus{{ title .Domain }} "{{ .Project }}/domain/{{ .Domain }}/cmd/scribus"

	"github.com/ribora/diogen/di"
)

func Init{{ title .Domain }}Scribus() {
	di.Wire[scribus{{ title .Domain }}.{{ title .Domain }}Management]({{ camel .Domain }}ManagementInstance)
}
