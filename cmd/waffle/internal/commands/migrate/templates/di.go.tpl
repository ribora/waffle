package di

import (
    {{ camel .Domain }}Migration "{{ .Project }}/domain/{{ .Domain }}/cmd/migrate"

    "github.com/ribora/diogen/di"
    "github.com/ribora/waffle/tools/migrate"
)

func Init{{ title .Domain }}Migrate() {
    di.Define({{ camel .Domain }}Migration.NewMigrations, di.Tag(migrate.MigrationsTag))
}
