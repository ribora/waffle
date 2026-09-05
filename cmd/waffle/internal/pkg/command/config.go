package command

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

// Config mirrors waffle.yaml.
type Config struct {
	ApiImport    string             `yaml:"api_import"`
	ProtoDir     string             `yaml:"proto_dir"`
	Mapper       string             `yaml:"mapper"`
	Replacements ReplacementsConfig `yaml:"replacements"`
}

// ReplacementsConfig is the replacements: group — a permission list, so that
// what nobody granted stays unreplaceable instead of merely unmentioned.
type ReplacementsConfig struct {
	// Keyed by import path, so one entry covers one package: the interfaces a
	// test may supply on the container's Replacements struct, in whatever form
	// the test author picks.
	Allow map[string][]string `yaml:"allow"`
}

// ProjectConfig loads waffle.yaml; a missing file is identical to all-defaults.
func (c *Command) ProjectConfig(projectName string) (Config, error) {
	root, err := findModuleRoot()
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	fh, err := os.Open(filepath.Join(root, "waffle.yaml"))
	if err == nil {
		//nolint:errcheck
		defer fh.Close()
		dec := yaml.NewDecoder(fh)
		dec.KnownFields(true)
		if err := dec.Decode(&cfg); err != nil {
			return Config{}, fmt.Errorf("failed to parse waffle.yaml: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("failed to open waffle.yaml: %w", err)
	}

	if cfg.ApiImport == "" {
		cfg.ApiImport = projectName + "/api/pb"
	}
	if cfg.ProtoDir == "" {
		cfg.ProtoDir = "api/proto"
	}
	// goverter by default: it fails generation on a field it cannot map, so a
	// field added to a model cannot silently stop reaching the wire. A
	// hand-written mapper loses that check and is opt-in.
	if cfg.Mapper == "" {
		cfg.Mapper = "goverter"
	}

	if cfg.Mapper != "dto" && cfg.Mapper != "goverter" {
		return Config{}, fmt.Errorf("waffle.yaml: mapper must be \"dto\" or \"goverter\", got %q", cfg.Mapper)
	}
	if err := cfg.Replacements.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Allowed reports whether a test may supply this interface itself.
func (r ReplacementsConfig) Allowed(path, name string) bool {
	return slices.Contains(r.Allow[path], name)
}

// A typo here would otherwise read as a denial: the check would name a seam the
// list looks like it already grants.
func (r ReplacementsConfig) validate() error {
	for path, names := range r.Allow {
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
