package command

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// EnsureTool adds module via `go get -tool` only if go.mod doesn't already
// list it — safe (and network-free) to call on every scaffold.
func (p *Project) EnsureTool(module string) error {
	return p.ensureTool(module, "latest")
}

// EnsureBuiltTool installs module at the version of owner this waffle was built
// with, so a new service does not get a generator newer than the waffle that scaffolds it.
func (p *Project) EnsureBuiltTool(module, owner string) error {
	return p.ensureTool(module, builtVersion(owner))
}

func builtVersion(owner string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "latest"
	}
	version := ""
	if info.Main.Path == owner {
		version = info.Main.Version
	}
	for _, dep := range info.Deps {
		if dep.Path != owner {
			continue
		}
		if dep.Replace != nil {
			dep = dep.Replace
		}
		version = dep.Version
	}
	// A local build stamps (devel), +dirty or a pseudo-version of a commit nobody can fetch.
	if !semver.IsValid(version) || semver.Build(version) != "" || module.IsPseudoVersion(version) {
		return "latest"
	}
	return version
}

func (p *Project) ensureTool(module, version string) error {
	data, err := os.ReadFile(p.Path("go.mod"))
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}
	if strings.Contains(string(data), module) {
		return nil
	}
	if _, err := p.Run("go", "get", "-tool", module+"@"+version); err != nil {
		return err
	}
	reportAction("added tool " + module)
	return nil
}
