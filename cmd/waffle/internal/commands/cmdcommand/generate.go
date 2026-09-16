package cmdcommand

import (
	"path/filepath"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

// Generate writes the command's package and its DI registration. Separate from
// Run so a test can call it without a Makefile or a module proxy.
func Generate(domain string, name command.CommandName, projectName string) error {
	c := command.New("command-internal", "", templates, nil)
	params := map[string]any{
		"Domain":  domain,
		"Command": name,
		"Project": projectName,
	}

	sources := map[string]string{
		filepath.Join("templates", "main.go.tpl"): filepath.Join("domain", domain, "cmd", name.Raw, "main.go"),
		filepath.Join("templates", "di.go.tpl"):   filepath.Join("config", "di", "di_"+domain+"_"+name.Snake+".go"),
	}
	return c.Write(sources, params)
}
