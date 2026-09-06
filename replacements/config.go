package replacements

import (
	"errors"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Allow map[string][]string `yaml:"allow"`
}

func (c Config) Allowed(path, name string) bool {
	return slices.Contains(c.Allow[path], name)
}

// A typo would otherwise read as a denial: the check names a seam the list looks
// like it already grants.
func (c Config) Validate() error {
	for path, names := range c.Allow {
		if strings.TrimSpace(path) == "" {
			return errors.New("waffle.yaml: replacements.allow has an entry with an empty import path")
		}
		if len(names) == 0 {
			return fmt.Errorf("waffle.yaml: replacements.allow lists %s with no interfaces", path)
		}
		for _, name := range names {
			if !token.IsIdentifier(name) || !token.IsExported(name) {
				return fmt.Errorf("waffle.yaml: replacements.allow lists %q under %s, which is not an exported Go identifier",
					name, path)
			}
		}
	}
	return nil
}

type projectFile struct {
	Replacements Config `yaml:"replacements"`
}

// Absent grants nothing: no file, or no key, is an answer, and the answer is no.
func LoadConfig(root string) (Config, error) {
	fh, err := os.Open(filepath.Join(root, "waffle.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("failed to open waffle.yaml: %w", err)
	}
	//nolint:errcheck
	defer fh.Close()

	// Not KnownFields: this reader is interested in one group, and the rest of
	// the file belongs to generators it must not have an opinion about.
	var file projectFile
	if err := yaml.NewDecoder(fh).Decode(&file); err != nil {
		return Config{}, fmt.Errorf("failed to parse waffle.yaml: %w", err)
	}
	if err := file.Replacements.Validate(); err != nil {
		return Config{}, err
	}
	return file.Replacements, nil
}

func Module(root string) (string, error) {
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

// Walks up, so a test may call the check from wherever its package sits.
func ModuleRoot() (string, error) {
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
