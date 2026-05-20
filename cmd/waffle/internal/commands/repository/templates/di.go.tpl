package di

import (
	"{{ .Project }}/domain/{{ .Domain }}/adapter/postgres"
	"{{ .Project }}/domain/{{ .Domain }}/service"

	"gitlab.com/gorib/di"
)

func Init{{ title .Domain }}{{ title .Entity }}Repository() {
	di.Wire[service.{{ title .Entity }}Repository](postgres.New{{ title .Entity }}Repository)
}
