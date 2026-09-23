// Package agents scaffolds AGENTS.md with project-wide AI-agent instructions
// for gorib/waffle services. Generated content is the same across services
// (framework conventions); service-specific context (module path, domains,
// modules, env vars) is auto-detected from the project state.
//
// AGENTS.md is regenerated on every run — any hand edits will be lost. Put
// service-specific instructions in CONTRIBUTING.md instead.
package agents

import (
	"bytes"
	"embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gitlab.com/gorib/waffle/cmd/waffle/internal/pkg/command"
	"gitlab.com/gorib/waffle/internal/gomod"
)

//go:embed templates/*
var templates embed.FS

// Generate writes AGENTS.md in the current working directory, auto-detecting
// the service context (domains, active waffle modules, env vars). projectName
// is the module path. quiet leaves out the files that were already current. It
// reports whether anything on disk changed.
func Generate(projectName string, quiet bool) (bool, error) {
	c := command.New("agents-internal", "", templates, nil)
	cfg, err := c.ProjectConfig(projectName)
	if err != nil {
		return false, err
	}
	areas, err := Areas(cfg.Agents)
	if err != nil {
		return false, err
	}
	out, err := c.Render(filepath.Join("templates", "AGENTS.md.tpl"), scanContext(projectName, areas))
	if err != nil {
		return false, err
	}

	changed := false
	if current, err := os.ReadFile("AGENTS.md"); err != nil || !bytes.Equal(current, out) {
		if err := os.WriteFile("AGENTS.md", out, 0644); err != nil {
			return false, fmt.Errorf("write AGENTS.md: %w", err)
		}
		command.ReportCreated("AGENTS.md")
		changed = true
	} else if !quiet {
		command.ReportUnchanged("AGENTS.md")
	}
	linked, err := ensureSymlinks(quiet)
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

func scanContext(projectName string, areas []Area) serviceContext {
	modules := scanModules()
	return serviceContext{
		ModulePath:    projectName,
		WaffleVersion: gomod.Requires(".", wafflePath),
		Domains:       scanDomains(),
		Modules:       modules,
		EnvVars:       scanEnvVars(modules),
		Modes:         modes,
		Areas:         areas,
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

const (
	wafflePath   = "gitlab.com/gorib/waffle"
	modulePrefix = wafflePath + "/config/"
)

// Parsed, not matched: the scaffolded modules.go ships with seven commented-out
// blank imports, and a regexp counts them as active.
func scanModules() []string {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("config", "di", "modules.go"), nil,
		parser.ImportsOnly|parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var modules []string
	for _, spec := range file.Imports {
		if spec.Name == nil || spec.Name.Name != "_" {
			continue
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
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

const envPackage = "gitlab.com/gorib/env"

// Every reader takes the key first, so the list is the package's, not a selection.
var envReaders = map[string]bool{
	"Value": true, "Array": true, "Json": true,
	"NeedValue": true, "NeedArray": true, "NeedJson": true,
	"MustValue": true, "MustArray": true, "MustJson": true,
}

// Parsed, not matched, for the same reason as scanModules.
func scanEnvVars(modules []string) []string {
	seen := map[string]bool{}
	for _, module := range modules {
		for _, name := range command.ModuleEnv(module) {
			seen[name] = true
		}
	}
	for _, root := range []string{"config", "domain", "cmd"} {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			switch {
			case err != nil, info.IsDir(), !strings.HasSuffix(path, ".go"):
				return nil
			case strings.HasSuffix(path, "_gen.go"), strings.HasSuffix(path, "_test.go"):
				return nil
			}
			// A file mid-edit was skipped before this parsed too; AGENTS.md is
			// not the place to report that it does not compile.
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
			if err != nil {
				return nil
			}
			readEnvVars(file, seen)
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

func readEnvVars(file *ast.File, seen map[string]bool) {
	local := ""
	for _, spec := range file.Imports {
		if p, err := strconv.Unquote(spec.Path.Value); err != nil || p != envPackage {
			continue
		}
		local = "env"
		if spec.Name != nil {
			local = spec.Name.Name
		}
	}
	if local == "" || local == "_" {
		return
	}

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		// env.NeedValue[string]("X") instantiates before it calls.
		fun := call.Fun
		switch indexed := fun.(type) {
		case *ast.IndexExpr:
			fun = indexed.X
		case *ast.IndexListExpr:
			fun = indexed.X
		}
		sel, ok := fun.(*ast.SelectorExpr)
		if !ok || !envReaders[sel.Sel.Name] {
			return true
		}
		if qualifier, ok := sel.X.(*ast.Ident); !ok || qualifier.Name != local {
			return true
		}
		key, ok := call.Args[0].(*ast.BasicLit)
		if !ok || key.Kind != token.STRING {
			return true
		}
		if name, err := strconv.Unquote(key.Value); err == nil && name != "" {
			seen[name] = true
		}
		return true
	})
}

func ensureSymlinks(quiet bool) (bool, error) {
	changed := false
	for _, name := range []string{"CLAUDE.md", ".cursorrules"} {
		if info, err := os.Lstat(name); err == nil {
			// Only replace existing symlinks; never overwrite real files.
			if info.Mode()&os.ModeSymlink == 0 {
				if !quiet {
					command.ReportKept(name)
				}
				continue
			}
			if target, err := os.Readlink(name); err == nil && target == "AGENTS.md" {
				if !quiet {
					command.ReportUnchanged(name)
				}
				continue
			}
			_ = os.Remove(name)
		}
		if err := os.Symlink("AGENTS.md", name); err != nil {
			return changed, fmt.Errorf("symlink %s: %w", name, err)
		}
		command.ReportCreated(name + " → AGENTS.md")
		changed = true
	}
	return changed, nil
}
