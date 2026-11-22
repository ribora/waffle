package domain

import (
	"path/filepath"

	"github.com/ribora/waffle/cmd/waffle/internal/commands/migrate"
	"github.com/ribora/waffle/cmd/waffle/internal/commands/repository"
	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command"
)

// Generate writes every file a domain and its own entity get. Separate from Run
// so the set can be produced without a Makefile, a module proxy or a tool
// install — which is what lets a test compare it against what the docs claim.
func Generate(p *command.Project, domain string, withRepository bool) error {
	params := map[string]any{
		"Domain":         domain,
		"Project":        p.Module(),
		"WithRepository": withRepository,
	}

	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "service", "entity.go.tpl"): filepath.Join(path, "service", "service.go"),
		filepath.Join("templates", "di.go.tpl"):                filepath.Join("config", "di", "di_"+domain+"_domain.go"),
	}
	if err := p.Write(templates, sources, params); err != nil {
		return err
	}

	// The domain's own entity goes through the path every later one takes.
	if !withRepository {
		return repository.WriteModel(p, domain, domain)
	}
	if err := repository.GenerateEntity(p, domain, domain); err != nil {
		return err
	}
	return migrate.Generate(p, domain, domain)
}
