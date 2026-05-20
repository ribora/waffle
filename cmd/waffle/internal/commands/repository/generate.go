package repository

import (
	"embed"
	"path/filepath"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templates embed.FS

// GenerateForDomain writes the postgres adapter files for the primary
// repository of a domain (entity == domain). Model, errors, service and the
// main domain DI file are owned by the domain package and not produced here.
func GenerateForDomain(domain, projectName string) error {
	return write(domain, domain, projectName, false)
}

// GenerateSecondary writes a full secondary repository (model + postgres +
// service interface + di) on top of an existing domain.
func GenerateSecondary(domain, entity, projectName string) error {
	return write(domain, entity, projectName, true)
}

func write(domain, entity, projectName string, withModelAndInterface bool) error {
	c := command.New("repository-internal", "", templates, nil)
	params := map[string]string{
		"Domain":  domain,
		"Entity":  entity,
		"Project": projectName,
	}
	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "postgres", "entity.go.tpl"): filepath.Join(path, "adapter", "postgres", entity+".go"),
		filepath.Join("templates", "postgres", "stored.go.tpl"): filepath.Join(path, "adapter", "postgres", "stored_"+entity+".go"),
		filepath.Join("templates", "di.go.tpl"):                 filepath.Join("config", "di", "di_"+domain+"_"+entity+"_repository.go"),
	}
	if withModelAndInterface {
		sources[filepath.Join("templates", "model", "entity.go.tpl")] = filepath.Join(path, "model", entity+".go")
		sources[filepath.Join("templates", "service", "repository.go.tpl")] = filepath.Join(path, "service", entity+"_repository.go")
	}
	return c.Write(sources, params)
}
