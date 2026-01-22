package config

import (
    "{{ .Project }}/domain/{{ .Domain }}/adapter/postgres"
    "{{ .Project }}/domain/{{ .Domain }}/service"

    "gitlab.com/gorib/di"
)

func Init{{ title .Domain }}Domain() {
    di.Wire[service.{{ title .Domain }}Repository](postgres.New{{ title .Domain }}Repository)
    {{ .Domain }}ManagementInstance := di.Define(service.New{{ title .Domain }}Management)
}
