package di

import (
	scribus{{ title .Domain }} "{{ .Project }}/domain/{{ .Domain }}/cmd/scribus"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/tools/scribus"
)

func Init{{ title .Domain }}{{ title .Event }}Scribus() {
	di.Define(scribus{{ title .Domain }}.New{{ title .Event }}Command, di.Tag(scribus.CommandTag))
}
