package di

import (
	scribus{{ title .Domain }} "{{ .Project }}/domain/{{ .Domain }}/cmd/scribus"

	"github.com/ribora/diogen/di"
	"github.com/ribora/waffle/tools/scribus"
)

func Init{{ title .Domain }}{{ title .Event }}Scribus() {
	di.Wire[scribus.BatchHandler](scribus{{ title .Domain }}.New{{ title .Event }}Command)
}
