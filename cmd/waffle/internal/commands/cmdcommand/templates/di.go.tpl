package di

import (
	cmd{{ .Command.Upper }} "{{ .Project }}/domain/{{ .Domain }}/cmd/{{ .Command.Raw }}"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
)

func Init{{ title .Domain }}{{ .Command.Upper }}() {
	di.Wire[cmd{{ .Command.Upper }}.{{ title .Domain }}Management]({{ camel .Domain }}ManagementInstance)
	di.Define(cmd{{ .Command.Upper }}.New{{ .Command.Upper }}Command, di.Tag(app.CommandTag))
}
