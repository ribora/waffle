// Package agents scaffolds AGENTS.md with project-wide AI-agent instructions
// for gorib/waffle services. Generated content is the same across services
// (framework conventions); service-specific context (module path, domains,
// modules, env vars) is auto-detected from the project state.
//
// AGENTS.md is regenerated on every run — any hand edits will be lost. Put
// service-specific instructions in CONTRIBUTING.md instead.
package agents

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/envread"
	"gitlab.com/gorib/waffle/internal/gomod"
)

//go:embed templates/*
var templateFiles embed.FS

var templates = command.NewTemplates(templateFiles)

// Generate writes AGENTS.md at the module root, auto-detecting the service
// context (domains, active waffle modules, env vars). quiet leaves out the files
// that were already current. It reports whether anything on disk changed.
func Generate(p *command.Project, quiet bool) (bool, error) {
	cfg, err := p.Config()
	if err != nil {
		return false, err
	}
	areas, err := Areas(cfg)
	if err != nil {
		return false, err
	}
	out, err := templates.Render(filepath.Join("templates", "AGENTS.md.tpl"), scanContext(p, areas))
	if err != nil {
		return false, err
	}

	changed, err := command.WriteIfChanged(p.Path("AGENTS.md"), out)
	if err != nil {
		return false, fmt.Errorf("write AGENTS.md: %w", err)
	}
	if changed {
		command.ReportCreated("AGENTS.md")
	} else if !quiet {
		command.ReportUnchanged("AGENTS.md")
	}
	linked, err := ensureSymlinks(p, quiet)
	return changed || linked, err
}

type serviceContext struct {
	ModulePath    string
	WaffleVersion string
	Domains       []string
	Modules       []string
	EnvVars       []string
	Modes         []Mode
	Areas         []Area
}

func scanContext(p *command.Project, areas []Area) serviceContext {
	modules := scanModules(p)
	return serviceContext{
		ModulePath:    p.Module(),
		WaffleVersion: gomod.Requires(p.Root(), wafflePath),
		Domains:       scanDomains(p),
		Modules:       modules,
		EnvVars:       scanEnvVars(p.Root(), modules),
		Modes:         modes,
		Areas:         areas,
	}
}

func scanDomains(p *command.Project) []string {
	entries, err := os.ReadDir(p.Path("domain"))
	if err != nil {
		return nil
	}
	var domains []string
	for _, e := range entries {
		if e.IsDir() {
			domains = append(domains, e.Name())
		}
	}
	sort.Strings(domains)
	return domains
}

const (
	wafflePath   = "gitlab.com/gorib/waffle"
	modulePrefix = wafflePath + "/config/"
)

func scanModules(p *command.Project) []string {
	active, err := p.ActiveModules()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var modules []string
	for _, path := range active {
		name, ok := strings.CutPrefix(path, modulePrefix)
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		modules = append(modules, name)
	}
	sort.Strings(modules)
	return modules
}

func scanEnvVars(root string, modules []string) []string {
	seen := map[string]bool{}
	for _, module := range modules {
		for _, name := range command.ModuleEnv(module) {
			seen[name] = true
		}
	}
	for _, read := range envread.Module(root) {
		seen[read.Key] = true
	}
	var vars []string
	for k := range seen {
		vars = append(vars, k)
	}
	sort.Strings(vars)
	return vars
}

func ensureSymlinks(p *command.Project, quiet bool) (bool, error) {
	changed := false
	for _, name := range []string{"CLAUDE.md", ".cursorrules"} {
		link := p.Path(name)
		if info, err := os.Lstat(link); err == nil {
			// Only replace existing symlinks; never overwrite real files.
			if info.Mode()&os.ModeSymlink == 0 {
				if !quiet {
					command.ReportKept(name)
				}
				continue
			}
			if target, err := os.Readlink(link); err == nil && target == "AGENTS.md" {
				if !quiet {
					command.ReportUnchanged(name)
				}
				continue
			}
			_ = os.Remove(link)
		}
		if err := os.Symlink("AGENTS.md", link); err != nil {
			return changed, fmt.Errorf("symlink %s: %w", name, err)
		}
		command.ReportCreated(name + " → AGENTS.md")
		changed = true
	}
	return changed, nil
}
