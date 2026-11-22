package di

import (
    "{{ .Project }}/domain/{{ .Domain }}/service"

    "github.com/ribora/diogen/di"
)

var {{ camel .Domain }}ManagementInstance di.Handle

func Init{{ title .Domain }}Domain() {
    {{ camel .Domain }}ManagementInstance = di.Define(service.New{{ title .Domain }}Management)
}
