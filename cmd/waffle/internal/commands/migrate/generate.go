package migrate

import (
	"fmt"
	"path/filepath"
	"time"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/plural"
)

// Generate writes the migration runner (cmd/migrate/main.go), the bootstrap
// SQL files and the DI registration for a domain.
func Generate(domain, projectName string) error {
	c := command.New("migrate-internal", "", templates, nil)
	service := filepath.Base(projectName)
	params := map[string]string{
		"Domain":  domain,
		"Project": projectName,
		"Service": service,
	}
	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "entity", "main.go.tpl"):        filepath.Join(path, "cmd", "migrate", "main.go"),
		filepath.Join("templates", "entity", "updated_at.sql.tpl"): filepath.Join(path, "cmd", "migrate", "migrations", fmt.Sprintf("%s00_create_update_at_function.sql", time.Now().Format("200601021504"))),
		filepath.Join("templates", "entity", "migration.sql.tpl"):  filepath.Join(path, "cmd", "migrate", "migrations", fmt.Sprintf("%s_create_%s_table.sql", time.Now().Format("20060102150405"), plural.Plural(domain))),
		filepath.Join("templates", "di.go.tpl"):                    filepath.Join("config", "di", "di_"+domain+"_migrate.go"),
	}
	return c.Write(sources, params)
}
