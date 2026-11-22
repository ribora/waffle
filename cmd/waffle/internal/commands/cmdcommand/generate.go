package cmdcommand

import (
	"path/filepath"

	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command"
)

// Generate writes the command's package and its DI registration. Separate from
// Run so a test can call it without a Makefile or a module proxy.
func Generate(p *command.Project, domain string, name command.CommandName) error {
	params := map[string]any{
		"Domain":  domain,
		"Command": name,
		"Project": p.Module(),
	}

	sources := map[string]string{
		filepath.Join("templates", "main.go.tpl"): filepath.Join("domain", domain, "cmd", name.Raw, "main.go"),
		filepath.Join("templates", "di.go.tpl"):   filepath.Join("config", "di", "di_"+domain+"_"+name.Snake+".go"),
	}
	return p.Write(templates, sources, params)
}
