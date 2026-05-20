package config

import (
    "{{ .Project }}/domain/{{ .Domain }}/service"

    "gitlab.com/gorib/di"
)

var {{ .Domain }}ManagementInstance any

func Init{{ title .Domain }}Domain() {
    {{ .Domain }}ManagementInstance = di.Define(service.New{{ title .Domain }}Management)
}
