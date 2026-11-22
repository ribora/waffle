package di

import (
	"{{ .Project }}/domain/{{ .Domain }}/adapter/postgres"
	"{{ .Project }}/domain/{{ .Domain }}/service"

	"github.com/ribora/diogen/di"
)

func Init{{ title .Domain }}{{ title .Entity }}Repository() {
	di.Wire[service.{{ title .Entity }}Repository](postgres.New{{ title .Entity }}Repository)
}
