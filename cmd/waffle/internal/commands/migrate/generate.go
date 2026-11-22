package migrate

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command"
	"github.com/ribora/waffle/cmd/waffle/internal/pkg/plural"
)

// Generate writes the migration runner (cmd/migrate/main.go) and the DI
// registration, which are per domain, plus the table for one entity. The runner
// and the trigger function are found by suffix, so every entity after the first
// adds its table and keeps the rest.
func Generate(p *command.Project, domain, entity string) error {
	params := Params(domain, entity, p.Module())
	sources := RunnerSources(domain)

	dir := Dir(domain)
	for _, m := range entityMigrations(entity, time.Now()) {
		if err := m.add(p, sources, dir); err != nil {
			return err
		}
	}
	return p.Write(templates, sources, params)
}

// For a caller that brings its own table and wants neither the entity template
// nor the updated_at trigger — the outbox is one.
func Runner(p *command.Project, domain string) error {
	return p.Write(templates, RunnerSources(domain), Params(domain, domain, p.Module()))
}

// RunnerSources and Params are exported for the same caller, so the runner it
// gets is the one every other domain has rather than a copy that drifts.
func RunnerSources(domain string) map[string]string {
	return map[string]string{
		filepath.Join("templates", "entity", "main.go.tpl"): filepath.Join("domain", domain, "cmd", "migrate", "main.go"),
		filepath.Join("templates", "di.go.tpl"):             filepath.Join("config", "di", "di_"+domain+"_migrate.go"),
	}
}

func Params(domain, entity, projectName string) map[string]string {
	return map[string]string{
		"Domain":  domain,
		"Entity":  entity,
		"Project": projectName,
		"Service": filepath.Base(projectName),
	}
}

func Dir(domain string) string {
	return filepath.Join("domain", domain, "cmd", "migrate", "migrations")
}

// The name carries a timestamp, so on the next run it is a different name and the
// existing file would not be recognised — a second migration for the same table.
// What identifies it is the part that does not move.
type migration struct {
	template string
	suffix   string
	name     string
}

func (m migration) add(p *command.Project, sources map[string]string, dir string) error {
	found, err := p.Glob(filepath.Join(dir, "*"+m.suffix))
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

func entityMigrations(entity string, now time.Time) []migration {
	return []migration{
		{
			template: filepath.Join("templates", "entity", "updated_at.sql.tpl"),
			suffix:   "_create_update_at_function.sql",
			name:     fmt.Sprintf("%s_create_update_at_function.sql", now.Add(-time.Second).Format("20060102150405")),
		},
		{
			template: filepath.Join("templates", "entity", "migration.sql.tpl"),
			suffix:   fmt.Sprintf("_create_%s_table.sql", plural.Plural(entity)),
			name:     fmt.Sprintf("%s_create_%s_table.sql", now.Format("20060102150405"), plural.Plural(entity)),
		},
	}
}
