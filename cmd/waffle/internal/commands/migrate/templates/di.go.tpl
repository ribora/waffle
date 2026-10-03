package di

import (
    {{ camel .Domain }}Migration "{{ .Project }}/domain/{{ .Domain }}/cmd/migrate"

    "gitlab.com/gorib/di"
    "gitlab.com/gorib/waffle/tools/migrate"
)

func Init{{ title .Domain }}Migrate() {
    di.Define({{ camel .Domain }}Migration.NewMigrations, di.Tag(migrate.MigrationsTag))
}
