package config

import (
    migration "{{ .Project }}/domain/{{ .Domain }}/cmd/migrate"

    "gitlab.com/gorib/di"
    "gitlab.com/gorib/waffle/tools/migrate"
)

func Init{{ title .Domain }}Migrate() {
    di.MustWire[migration.Migration](migration.NewMigrations, di.Tag(migrate.MigrationsTag))
}
