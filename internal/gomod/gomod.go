package gomod

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

func Root() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("failed to stat go.mod: %w", err)
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", errors.New("failed to find go.mod")
		}
		cwd = parent
	}
}

// Empty for a module that is replaced: the replacement is a path on one
// machine, and there is no released version to name.
func Requires(root, module string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	parsed, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return ""
	}
	for _, replaced := range parsed.Replace {
		if replaced.Old.Path == module {
			return ""
		}
	}
	for _, required := range parsed.Require {
		if required.Mod.Path == module {
			return required.Mod.Version
		}
	}
	return ""
}

func Path(root string) (string, error) {
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}
	for line := range strings.Lines(string(goMod)) {
		if p, found := strings.CutPrefix(strings.TrimSpace(line), "module "); found {
			return strings.TrimSpace(p), nil
		}
	}
	return "", errors.New("go.mod declares no module path")
}
