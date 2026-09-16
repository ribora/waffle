package repository

import (
	"embed"
	"path/filepath"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templates embed.FS

// WriteModel writes the domain model for an entity, and nothing else. It is the
// first link of the chain: the repository needs the model, the model needs
// nothing, so a domain without a repository still gets one.
func WriteModel(domain, entity, projectName string) error {
	return writeModel(newCommand(), domain, entity, params(domain, entity, projectName))
}

// GenerateEntity writes everything an entity of a domain gets, the first one and
// the tenth alike: model, repository interface, postgres adapter, stored struct
// and the DI registration. The migration is the caller's, because the runner is
// per domain rather than per entity.
func GenerateEntity(domain, entity, projectName string) error {
	c := newCommand()
	p := params(domain, entity, projectName)
	if err := writeModel(c, domain, entity, p); err != nil {
		return err
	}
	path := filepath.Join("domain", domain)
	sources := map[string]string{
		filepath.Join("templates", "postgres", "entity.go.tpl"): filepath.Join(path, "adapter", "postgres", entity+".go"),
		filepath.Join("templates", "postgres", "stored.go.tpl"): filepath.Join(path, "adapter", "postgres", "stored_"+entity+".go"),
		filepath.Join("templates", "di.go.tpl"):                 filepath.Join("config", "di", "di_"+domain+"_"+entity+"_repository.go"),
	}
	if err := c.Write(sources, p); err != nil {
		return err
	}
	return appendInterface(c, domain, entity, p)
}

func newCommand() *command.Command {
	return command.New("repository-internal", "", templates, nil)
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
func writeModel(c *command.Command, domain, entity string, params map[string]string) error {
	dir := filepath.Join("domain", domain, "model")
	declared := command.Title(entity)
	if where, found := command.Declares(dir, declared); found {
		command.KeptAs("model."+declared, where)
		return nil
	}
	return c.Write(map[string]string{
		filepath.Join("templates", "model", "entity.go.tpl"): filepath.Join(dir, entity+".go"),
	}, params)
}
