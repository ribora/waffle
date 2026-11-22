// Package agents writes waffle.md, the framework's conventions for a service and
// what the service is made of — module path, domains, modules, env vars, read off
// the project — and keeps the line in the service's AGENTS.md that brings it in.
//
// waffle.md is rewritten on every run, and an edit there is lost; AGENTS.md is the
// service's own.
package agents

import (
	"embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ribora/waffle/cmd/waffle/internal/pkg/command"
	"github.com/ribora/waffle/container"
	"github.com/ribora/waffle/internal/gomod"
)

//go:embed templates/*
var templateFiles embed.FS

var templates = command.NewTemplates(templateFiles)

const reference = "Read @waffle.md before the first edit: the framework's conventions, and what this service is made of."

// What every waffle.md before it wrote into AGENTS.md: such a file was waffle's,
// and it is the service's stub from now on.
const managed = "**managed by `./waffle agents`**"

// Generate writes waffle.md at the module root, auto-detecting the service
// context (domains, active waffle modules, env vars), and brings it into
// AGENTS.md. quiet leaves out the files that were already current. It reports
// whether anything on disk changed.
func Generate(p *command.Project, quiet bool) (bool, error) {
	out, err := templates.Render(filepath.Join("templates", "waffle.md.tpl"), scanContext(p))
	if err != nil {
		return false, err
	}

	changed, err := command.WriteIfChanged(p.Path("waffle.md"), out)
	if err != nil {
		return false, fmt.Errorf("write waffle.md: %w", err)
	}
	if changed {
		command.ReportCreated("waffle.md")
	} else if !quiet {
		command.ReportUnchanged("waffle.md")
	}
	brought, err := bringIn(p, quiet)
	if err != nil {
		return false, err
	}
	linked, err := ensureSymlinks(p, quiet)
	return changed || brought || linked, err
}

// AGENTS.md is the service's: written as a stub where there is none, given the
// line that brings waffle.md in where it lacks it, and otherwise left alone.
func bringIn(p *command.Project, quiet bool) (bool, error) {
	file := p.Path("AGENTS.md")
	text, err := os.ReadFile(file)
	switch {
	case os.IsNotExist(err), err == nil && strings.Contains(string(text), managed):
		stub := fmt.Sprintf("# AGENTS.md\n\nInstructions for AI coding agents working in %s.\n\n%s\n", path.Base(p.Module()), reference)
		if err := os.WriteFile(file, []byte(stub), 0o644); err != nil {
			return false, fmt.Errorf("write AGENTS.md: %w", err)
		}
		command.ReportCreated("AGENTS.md")
		return true, nil
	case err != nil:
		return false, err
	case strings.Contains(string(text), "@waffle.md"):
		if !quiet {
			command.ReportKept("AGENTS.md")
		}
		return false, nil
	}
	gap := "\n"
	if !strings.HasSuffix(string(text), "\n") {
		gap = "\n\n"
	}
	if err := os.WriteFile(file, append(text, []byte(gap+reference+"\n")...), 0o644); err != nil {
		return false, fmt.Errorf("write AGENTS.md: %w", err)
	}
	command.ReportAction("AGENTS.md: the line that brings waffle.md in, at its end")
	return true, nil
}

type serviceContext struct {
	ModulePath    string
	WaffleVersion string
	Domains       []string
	Modules       []string
	EnvVars       []string
}

func scanContext(p *command.Project) serviceContext {
	modules := scanModules(p)
	return serviceContext{
		ModulePath:    p.Module(),
		WaffleVersion: gomod.Requires(p.Root(), wafflePath),
		Domains:       scanDomains(p),
		Modules:       modules,
		EnvVars:       scanEnvVars(p.Root(), modules),
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
	wafflePath   = "github.com/ribora/waffle"
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
	for _, read := range container.EnvReads(root) {
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
