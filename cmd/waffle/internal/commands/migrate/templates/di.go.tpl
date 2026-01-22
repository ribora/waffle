package config

import (
    {{ .Domain }}Migration "{{ .Project }}/domain/{{ .Domain }}/cmd/migrate"

    "gitlab.com/gorib/di"
    "gitlab.com/gorib/waffle/tools/migrate"
)

func Init{{ title .Domain }}Migrate() {
    di.Wire[{{ .Domain }}Migration.Migration]({{ .Domain }}Migration.NewMigrations, di.Tag(migrate.MigrationsTag))
}
