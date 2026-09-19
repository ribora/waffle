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
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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

const modulePrefix = "gitlab.com/gorib/waffle/config/"

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

// Every reader takes the key first, so the list is the package's, not a
// selection: the regexp this replaces knew four of the nine.
var envReaders = map[string]bool{
	"Value": true, "Array": true, "Json": true,
	"NeedValue": true, "NeedArray": true, "NeedJson": true,
	"MustValue": true, "MustArray": true, "MustJson": true,
}

// Parsed, not matched, for the same reason as scanModules.
func scanEnvVars() []string {
	seen := map[string]bool{}
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
