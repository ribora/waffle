package migrate

import (
	"fmt"
	"path/filepath"
	"time"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/plural"
)

// Generate writes the migration runner (cmd/migrate/main.go) and the DI
// registration, which are per domain, plus the table for one entity. The runner
// and the trigger function are found by suffix, so every entity after the first
// adds its table and keeps the rest.
func Generate(domain, entity, projectName string) error {
	c := command.New("migrate-internal", "", templates, nil)
	service := filepath.Base(projectName)
	params := map[string]string{
		"Domain":  domain,
		"Entity":  entity,
		"Project": projectName,
		"Service": service,
	}
	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "entity", "main.go.tpl"): filepath.Join(path, "cmd", "migrate", "main.go"),
		filepath.Join("templates", "di.go.tpl"):             filepath.Join("config", "di", "di_"+domain+"_migrate.go"),
	}

	dir := filepath.Join(path, "cmd", "migrate", "migrations")
	now := time.Now()
	migrations := []migration{
		{
			template: filepath.Join("templates", "entity", "updated_at.sql.tpl"),
			suffix:   "_create_update_at_function.sql",
			name:     fmt.Sprintf("%s00_create_update_at_function.sql", now.Format("200601021504")),
		},
		{
			template: filepath.Join("templates", "entity", "migration.sql.tpl"),
			suffix:   fmt.Sprintf("_create_%s_table.sql", plural.Plural(entity)),
			name:     fmt.Sprintf("%s_create_%s_table.sql", now.Format("20060102150405"), plural.Plural(entity)),
		},
	}
	for _, m := range migrations {
		if err := m.add(sources, dir); err != nil {
			return err
		}
	}
	return c.Write(sources, params)
}

// The name carries a timestamp, so on the next run it is a different name and the
// existing file would not be recognised — a second migration for the same table.
// What identifies it is the part that does not move.
type migration struct {
	template string
	suffix   string
	name     string
}

func (m migration) add(sources map[string]string, dir string) error {
	found, err := filepath.Glob(filepath.Join(dir, "*"+m.suffix))
	if err != nil {
		return err
	}
	if len(found) > 0 {
		command.ReportKept(found...)
		return nil
	}
	sources[m.template] = filepath.Join(dir, m.name)
	return nil
}
