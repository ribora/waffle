package config

import (
    "{{ .Project }}/domain/{{ .Domain }}/adapter/postgres"
    "{{ .Project }}/domain/{{ .Domain }}/service"

    "gitlab.com/gorib/di"
)

func Init{{ title .Domain }}Domain() {
    di.MustWire[service.{{ title .Domain }}Repository](postgres.New{{ title .Domain }}Repository)
    di.MustDefine(service.New{{ title .Domain }}Management,
        di.Alias[](),
    )
}
