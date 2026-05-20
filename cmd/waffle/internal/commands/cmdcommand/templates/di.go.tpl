package config

import (
	cmd{{ title .Command }} "{{ .Project }}/domain/{{ .Domain }}/cmd/{{ .Command }}"

	"gitlab.com/gorib/di"
	"gitlab.com/gorib/waffle/app"
)

func Init{{ title .Domain }}{{ title .Command }}() {
	di.Wire[cmd{{ title .Command }}.{{ title .Domain }}Management]({{ .Domain }}ManagementInstance)
	di.Define(cmd{{ title .Command }}.New{{ title .Command }}Command, di.Tag(app.CommandTag))
}
