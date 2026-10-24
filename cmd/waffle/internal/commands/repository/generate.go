package repository

import (
	"embed"
	"path/filepath"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templateFiles embed.FS

var templates = command.NewTemplates(templateFiles)

// WriteModel writes the domain model for an entity, and nothing else. It is the
// first link of the chain: the repository needs the model, the model needs
// nothing, so a domain without a repository still gets one.
func WriteModel(p *command.Project, domain, entity string) error {
	return writeModel(p, domain, entity, params(domain, entity, p.Module()))
}

// GenerateEntity writes everything an entity of a domain gets, the first one and
// the tenth alike: model, repository interface, postgres adapter, stored struct
// and the DI registration. The migration is the caller's, because the runner is
// per domain rather than per entity.
func GenerateEntity(p *command.Project, domain, entity string) error {
	values := params(domain, entity, p.Module())
	if err := writeModel(p, domain, entity, values); err != nil {
		return err
	}
	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "postgres", "entity.go.tpl"): filepath.Join(path, "adapter", "postgres", entity+".go"),
		filepath.Join("templates", "postgres", "stored.go.tpl"): filepath.Join(path, "adapter", "postgres", "stored_"+entity+".go"),
		filepath.Join("templates", "di.go.tpl"):                 filepath.Join("config", "di", "di_"+domain+"_"+entity+"_repository.go"),
	}
	if err := p.Write(templates, sources, values); err != nil {
		return err
	}
	return appendInterface(p, domain, entity, values)
}

func params(domain, entity, projectName string) map[string]string {
	return map[string]string{
		"Domain":  domain,
		"Entity":  entity,
		"Project": projectName,
	}
}

// The model is the one generated declaration that shares a package with types a
// person writes by hand, so the name it would introduce has to be looked for
// across the package and not only at the path it would land on.
func writeModel(p *command.Project, domain, entity string, params map[string]string) error {
	dir := filepath.Join("domain", domain, "model")
	declared := command.Title(entity)
	if where, found := p.Declares(dir, declared); found {
		command.KeptAs("model."+declared, where)
		return nil
	}
	return p.Write(templates, map[string]string{
		filepath.Join("templates", "model", "entity.go.tpl"): filepath.Join(dir, entity+".go"),
	}, params)
}
