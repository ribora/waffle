package di

import (
    "{{ .Project }}/domain/{{ .Domain }}/service"

    "gitlab.com/gorib/di"
)

var {{ camel .Domain }}ManagementInstance any

func Init{{ title .Domain }}Domain() {
    {{ camel .Domain }}ManagementInstance = di.Define(service.New{{ title .Domain }}Management)
}
