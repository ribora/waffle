// Package agents scaffolds AGENTS.md with project-wide AI-agent instructions
// for gorib/waffle services. Generated content is the same across services
// (framework conventions); service-specific context (module path, domains,
// modules, env vars) is auto-detected from the project state.
//
// AGENTS.md is regenerated on every run — any hand edits will be lost. Put
// service-specific instructions in CONTRIBUTING.md instead.
package agents

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
)

//go:embed templates/*
var templates embed.FS

// Generate writes AGENTS.md in the current working directory, auto-detecting
// the service context (domains, active waffle modules, env vars). projectName
// is the module path. Overwrites existing AGENTS.md.
func Generate(projectName string) error {
	ctx := scanContext(projectName)

	c := command.New("agents-internal", "", templates, nil)
	sources := map[string]string{
		filepath.Join("templates", "AGENTS.md.tpl"): "AGENTS.md",
	}

	// Force regeneration: AGENTS.md is fully waffle-managed.
	_ = os.Remove("AGENTS.md")

	if err := c.Write(sources, ctx); err != nil {
		return err
	}
	return ensureSymlinks()
}

type serviceContext struct {
	ModulePath string
	Domains    []string
	Modules    []string
	EnvVars    []string
}

func scanContext(projectName string) serviceContext {
	return serviceContext{
		ModulePath: projectName,
		Domains:    scanDomains(),
		Modules:    scanModules(),
		EnvVars:    scanEnvVars(),
	}
}

func scanDomains() []string {
	entries, err := os.ReadDir("domain")
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

var blankImportRe = regexp.MustCompile(`_\s+"gitlab\.com/gorib/waffle/config/([^"]+)"`)

func scanModules() []string {
	data, err := os.ReadFile(filepath.Join("config", "di", "modules.go"))
	if err != nil {
		return nil
	}
	matches := blankImportRe.FindAllStringSubmatch(string(data), -1)
	seen := map[string]bool{}
	var modules []string
	for _, m := range matches {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		modules = append(modules, m[1])
	}
	sort.Strings(modules)
	return modules
}

var envVarRe = regexp.MustCompile(`env\.(?:Need)?(?:Value|Array)\b[^(]*\(\s*"([A-Z_][A-Z0-9_]*)"`)

func scanEnvVars() []string {
	seen := map[string]bool{}
	for _, root := range []string{"config", "domain", "cmd"} {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_gen.go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			//nolint:errcheck
			defer f.Close()
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				for _, m := range envVarRe.FindAllStringSubmatch(sc.Text(), -1) {
					seen[m[1]] = true
				}
			}
			return nil
		})
	}
	var vars []string
	for k := range seen {
		vars = append(vars, k)
	}
	sort.Strings(vars)
	return vars
}

func ensureSymlinks() error {
	for _, name := range []string{"CLAUDE.md", ".cursorrules"} {
		if info, err := os.Lstat(name); err == nil {
			// Only replace existing symlinks; never overwrite real files.
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			_ = os.Remove(name)
		}
		if err := os.Symlink("AGENTS.md", name); err != nil {
			return fmt.Errorf("symlink %s: %w", name, err)
		}
	}
	return nil
}
