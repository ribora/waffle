package domain

import (
	"path/filepath"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/migrate"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/commands/repository"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

// Generate writes every file a domain and its own entity get. Separate from Run
// so the set can be produced without a Makefile, a module proxy or a tool
// install — which is what lets a test compare it against what the docs claim.
func Generate(domain, projectName string, withRepository bool) error {
	c := command.New("domain-internal", "", templates, nil)
	params := map[string]any{
		"Domain":         domain,
		"Project":        projectName,
		"WithRepository": withRepository,
	}

	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "model", "errors.go.tpl"):   filepath.Join(path, "model", "errors.go"),
		filepath.Join("templates", "service", "entity.go.tpl"): filepath.Join(path, "service", "service.go"),
		filepath.Join("templates", "di.go.tpl"):                filepath.Join("config", "di", "di_"+domain+"_domain.go"),
	}
	if err := c.Write(sources, params); err != nil {
		return err
	}

	// The domain's own entity goes through the path every later one takes.
	if !withRepository {
		return repository.WriteModel(domain, domain, projectName)
	}
	if err := repository.GenerateEntity(domain, domain, projectName); err != nil {
		return err
	}
	return migrate.Generate(domain, domain, projectName)
}
